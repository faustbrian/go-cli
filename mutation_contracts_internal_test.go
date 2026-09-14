package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestCompilationExactBoundariesAndAllocatedIdentities(t *testing.T) {
	t.Parallel()

	firstArgument := StringArgument("first")
	secondArgument := StringArgument("second").Optional()
	firstOption := BoolOption("alpha")
	secondOption := BoolOption("zulu")
	root := NewCommand(
		"r",
		WithArguments(firstArgument, secondArgument),
		WithOptions(firstOption, secondOption),
	)
	application, err := Compile(root, WithLimits(Limits{
		MaximumCommandDepth: 1, MaximumCommands: 1,
		MaximumArgumentsPerCommand: 2, MaximumOptionsPerCommand: 2,
	}))
	if err != nil {
		t.Fatalf("Compile(exact root limits) error = %v", err)
	}
	if application.root.id != 0 ||
		application.root.arguments[0].key != 0 || application.root.arguments[1].key != 1 ||
		application.root.options[0].key != 2 || application.root.options[1].key != 3 {
		t.Fatalf("allocated identities = command %d, arguments %d/%d, options %d/%d",
			application.root.id,
			application.root.arguments[0].key, application.root.arguments[1].key,
			application.root.options[0].key, application.root.options[1].key,
		)
	}

	child := NewCommand("c", WithSubcommands(NewCommand("g")))
	tree, err := Compile(NewCommand("r", WithSubcommands(child)), WithLimits(Limits{
		MaximumCommandDepth: 3, MaximumCommands: 3,
	}))
	if err != nil {
		t.Fatalf("Compile(exact tree limits) error = %v", err)
	}
	if tree.root.id != 0 || tree.root.children[0].id != 1 || tree.root.children[0].children[0].id != 2 {
		t.Fatalf("command identities = %d/%d/%d",
			tree.root.id, tree.root.children[0].id, tree.root.children[0].children[0].id,
		)
	}

	if _, err := Compile(NewCommand("tool"), WithLimits(Limits{MaximumMetadataBytes: 4})); err != nil {
		t.Fatalf("Compile(exact metadata limit) error = %v", err)
	}
	for _, code := range []int{1, 255} {
		policy := ExitCodePolicy{Usage: code, Command: code, Canceled: code, Deadline: code, Internal: code}
		if _, err := Compile(NewCommand("tool"), WithExitCodePolicy(policy)); err != nil {
			t.Fatalf("Compile(exit code %d) error = %v", code, err)
		}
	}
}

func TestOptionGroupSatisfiabilityTraversesEveryComponent(t *testing.T) {
	t.Parallel()

	a, b, c := new(int), new(int), new(int)
	options := []optionSpec{
		{binding: a, required: true},
		{binding: b},
		{binding: c, required: true},
	}
	together := optionGroupSpec{kind: optionGroupTogether, bindings: []any{a, b}}
	optionalOptions := []optionSpec{{binding: a}, {binding: b}, {binding: c}}
	if err := validateGroupSatisfiability([]optionGroupSpec{
		together,
		{kind: optionGroupExclusive, bindings: []any{a, b}},
	}, optionalOptions); err != nil {
		t.Fatalf("exclusive duplicate within one component error = %v", err)
	}
	if err := validateGroupSatisfiability([]optionGroupSpec{
		together,
		{kind: optionGroupExclusive, bindings: []any{a, b, c}},
	}, options); !errors.Is(err, ErrInternal) {
		t.Fatalf("exclusive forced components error = %v", err)
	}
	if err := validateGroupSatisfiability([]optionGroupSpec{
		{kind: optionGroupExclusive, bindings: []any{a, c}},
		together,
	}, []optionSpec{{binding: a, required: true}, {binding: b}, {binding: c}}); err != nil {
		t.Fatalf("exclusive group processed as a union error = %v", err)
	}
	if err := validateGroupSatisfiability([]optionGroupSpec{
		{kind: optionGroupExclusive, bindings: []any{a, b}},
		together,
	}, []optionSpec{{binding: a, required: true}, {binding: b}}); !errors.Is(err, ErrInternal) {
		t.Fatalf("together group after exclusive group error = %v", err)
	}

	d := new(int)
	chainedOptions := append([]optionSpec(nil), options...)
	chainedOptions = append(chainedOptions, optionSpec{binding: d})
	if err := validateGroupSatisfiability([]optionGroupSpec{
		{kind: optionGroupTogether, bindings: []any{a, b}},
		{kind: optionGroupTogether, bindings: []any{b, d}},
		{kind: optionGroupExclusive, bindings: []any{a, d}},
	}, chainedOptions); !errors.Is(err, ErrInternal) {
		t.Fatalf("chained union error = %v", err)
	}
	if err := validateGroupSatisfiability([]optionGroupSpec{
		{kind: optionGroupTogether, bindings: []any{a, b}},
		{kind: optionGroupExclusive, bindings: []any{a, b, c, d}},
	}, []optionSpec{
		{binding: a}, {binding: b}, {binding: c, required: true}, {binding: d, required: true},
	}); !errors.Is(err, ErrInternal) {
		t.Fatalf("duplicate optional component traversal error = %v", err)
	}
}

func TestASCIIAlphaNumericIncludesOnlyExactRanges(t *testing.T) {
	t.Parallel()

	for _, character := range []rune{'a', 'z', 'A', 'Z', '0', '9'} {
		if !isASCIIAlphaNumeric(character) {
			t.Fatalf("isASCIIAlphaNumeric(%q) = false", character)
		}
	}
	for _, character := range []rune{'`', '{', '@', '[', '/', ':'} {
		if isASCIIAlphaNumeric(character) {
			t.Fatalf("isASCIIAlphaNumeric(%q) = true", character)
		}
	}
}

func TestCompletionPositionPreservesTokenGrammar(t *testing.T) {
	t.Parallel()

	value := optionSpec{name: "value", short: 'v'}
	boolean := optionSpec{name: "all", short: 'a', boolean: true}
	child := &compiledCommand{name: "child", aliases: []string{"alias"}}
	root := &compiledCommand{
		name: "tool", effective: []optionSpec{boolean, value}, children: []*compiledCommand{child},
	}

	for _, token := range []string{"", "value", "--value", "-"} {
		if option, attached, ok := completionAttachedShortOption(root, token); ok || option != nil || attached != "" {
			t.Fatalf("completionAttachedShortOption(%q) = %#v/%q/%v", token, option, attached, ok)
		}
	}
	if option, attached, ok := completionAttachedShortOption(root, "-avtail"); !ok || option == nil || option.name != "value" || attached != "tail" {
		t.Fatalf("completionAttachedShortOption(cluster) = %#v/%q/%v", option, attached, ok)
	}

	assertPosition := func(tokens []string, wantCommand *compiledCommand, wantPositional int, wantPending string) {
		t.Helper()
		command, positional, pending := completionPosition(root, tokens)
		pendingName := ""
		if pending != nil {
			pendingName = pending.name
		}
		if command != wantCommand || positional != wantPositional || pendingName != wantPending {
			t.Fatalf("completionPosition(%q) = %p/%d/%q, want %p/%d/%q",
				tokens, command, positional, pendingName, wantCommand, wantPositional, wantPending,
			)
		}
	}
	assertPosition([]string{"--", "one", "two"}, root, 2, "")
	assertPosition([]string{"positional", "--", "one", "two"}, root, 3, "")
	assertPosition([]string{"--value"}, root, 0, "value")
	assertPosition([]string{"--value=assigned"}, root, 0, "")
	assertPosition([]string{"--all"}, root, 0, "")
	assertPosition([]string{"-a"}, root, 0, "")
	assertPosition([]string{"-av"}, root, 0, "value")
	assertPosition([]string{"-vattached"}, root, 0, "")
	assertPosition([]string{"-vv"}, root, 0, "")
	assertPosition([]string{"-?"}, root, 1, "")
	assertPosition([]string{"-?a"}, root, 1, "")
	assertPosition([]string{"-a", "positional"}, root, 1, "")
	assertPosition([]string{"--value", "consumed", "positional"}, root, 1, "")
	assertPosition([]string{"positional", "alias"}, child, 0, "")
}

func TestCompletionArgumentAndCandidateBoundsAreExact(t *testing.T) {
	t.Parallel()

	for _, cardinality := range []ArgumentCardinality{ArgumentRepeated, ArgumentRemainder} {
		command := &compiledCommand{arguments: []argumentSpec{{name: "values", cardinality: cardinality}}}
		if argument := completionArgument(command, 3); argument == nil || argument.name != "values" {
			t.Fatalf("completionArgument(%d) = %#v", cardinality, argument)
		}
	}
	if argument := completionArgument(&compiledCommand{arguments: []argumentSpec{{
		name: "value", cardinality: ArgumentOptional,
	}}}, 1); argument != nil {
		t.Fatalf("completionArgument(optional overflow) = %#v", argument)
	}

	application := &Application{limits: Limits{
		MaximumCompletionProviderResults: 10,
		MaximumCompletionResults:         10,
		MaximumCompletionBytes:           6,
	}}
	bounded := application.boundCandidates([]CompletionCandidate{
		{Value: "oversized", Description: "value"},
		{Value: "exact", Description: "x"},
	})
	if len(bounded) != 1 || bounded[0].Value != "exact" {
		t.Fatalf("exact byte bounds = %#v", bounded)
	}
	bounded = application.boundCandidates([]CompletionCandidate{
		{Value: "a", Description: "12"},
		{Value: "b", Description: "3"},
		{Value: "c", Description: "4"},
	})
	if len(bounded) != 2 || bounded[0].Value != "a" || bounded[1].Value != "b" {
		t.Fatalf("cumulative byte bounds = %#v", bounded)
	}
	application.limits.MaximumCompletionResults = 1
	bounded = application.boundCandidates([]CompletionCandidate{{Value: "a"}, {Value: "b"}})
	if len(bounded) != 1 || bounded[0].Value != "a" {
		t.Fatalf("result count bounds = %#v", bounded)
	}
	application.limits.MaximumCompletionBytes = 1
	bounded = application.boundCandidates([]CompletionCandidate{
		{Value: string([]byte{0xff})},
		{Value: "a"},
	})
	if len(bounded) != 1 || bounded[0].Value != "a" {
		t.Fatalf("sanitized expansion bounds = %#v", bounded)
	}
	application.limits.MaximumCompletionResults = 10
	application.limits.MaximumCompletionBytes = 2
	bounded = application.boundCandidates([]CompletionCandidate{
		{Value: "x"},
		{Value: "\x1ba"},
	})
	if len(bounded) != 1 || bounded[0].Value != "x" {
		t.Fatalf("cumulative raw byte bounds = %#v", bounded)
	}
	for _, test := range []struct {
		name      string
		candidate CompletionCandidate
		limit     int
		want      bool
	}{
		{name: "empty exact", candidate: CompletionCandidate{}, limit: 0, want: true},
		{name: "value over", candidate: CompletionCandidate{Value: "a"}, limit: 0},
		{name: "description over", candidate: CompletionCandidate{Value: "a", Description: "bc"}, limit: 2},
		{name: "combined exact", candidate: CompletionCandidate{Value: "a", Description: "b"}, limit: 2, want: true},
	} {
		if got := completionCandidateWithinRawByteLimit(test.candidate, test.limit); got != test.want {
			t.Fatalf("completionCandidateWithinRawByteLimit(%s) = %t, want %t", test.name, got, test.want)
		}
	}
}

func TestCompletionSkipsHiddenChildrenAndPreservesDeadline(t *testing.T) {
	t.Parallel()

	application := &Application{
		root: &compiledCommand{children: []*compiledCommand{
			{name: "hidden", hidden: true},
			{name: "visible"},
		}},
		limits: defaultLimits(),
	}
	candidates, err := application.Complete(context.Background(), []string{""})
	if err != nil || len(candidates) != 1 || candidates[0].Value != "visible" {
		t.Fatalf("visible candidates = %#v, error = %v", candidates, err)
	}
	if _, err := application.dynamicCandidates(context.Background(), func(context.Context, CompletionRequest) ([]CompletionCandidate, error) {
		return nil, context.DeadlineExceeded
	}, CompletionRequest{}); !errors.Is(err, context.DeadlineExceeded) || errors.Is(err, ErrCompletion) {
		t.Fatalf("provider deadline error = %v", err)
	}
}

func TestHelpAndMarkdownRenderEveryIndependentSection(t *testing.T) {
	t.Parallel()

	for name, command := range map[string]*compiledCommand{
		"experimental": {name: "tool", experimental: true},
		"deprecated":   {name: "tool", deprecated: "old"},
		"replacement":  {name: "tool", replacement: "new"},
	} {
		help, err := (&Application{root: command}).Help(nil, HelpOptions{})
		if err != nil || !strings.Contains(help, "Status:\n") {
			t.Fatalf("Help(%s) = %q, %v", name, help, err)
		}
	}
	emptyHelp, err := (&Application{root: &compiledCommand{name: "tool"}}).Help(nil, HelpOptions{})
	if err != nil || strings.Contains(emptyHelp, "Status:") || strings.Contains(emptyHelp, "Arguments:") ||
		strings.Contains(emptyHelp, "Options:") || strings.Contains(emptyHelp, "Aliases:") ||
		strings.Contains(emptyHelp, "Examples:") || strings.Contains(emptyHelp, "[options]") {
		t.Fatalf("empty Help() = %q, %v", emptyHelp, err)
	}
	rich := &compiledCommand{
		name: "tool", aliases: []string{"t"}, examples: []string{"tool run"},
		children:  []*compiledCommand{{name: "hidden", hidden: true}, {name: "visible"}},
		arguments: []argumentSpec{{name: "value", cardinality: ArgumentRequired}},
		options:   []optionSpec{{name: "local", origin: "tool"}},
		effective: []optionSpec{{name: "local", origin: "tool"}},
	}
	richHelp, err := (&Application{root: rich}).Help(nil, HelpOptions{})
	if err != nil || !strings.Contains(richHelp, "visible") || strings.Contains(richHelp, "hidden") ||
		!strings.Contains(richHelp, "Arguments:") || !strings.Contains(richHelp, "Options:") ||
		!strings.Contains(richHelp, "Aliases:") || !strings.Contains(richHelp, "Examples:") {
		t.Fatalf("rich Help() = %q, %v", richHelp, err)
	}

	for name, command := range map[string]*compiledCommand{
		"experimental": {name: "tool", experimental: true},
		"hidden":       {name: "tool", hidden: true},
		"deprecated":   {name: "tool", deprecated: "old"},
		"replacement":  {name: "tool", replacement: "new"},
	} {
		var output strings.Builder
		writeMarkdownCommand(&output, command, "tool", 2)
		if !strings.Contains(output.String(), "### Status\n") {
			t.Fatalf("Markdown(%s) = %q", name, output.String())
		}
	}
	markdownCommand := &compiledCommand{
		name: "tool", summary: "summary", description: "description",
		aliases: []string{"t"}, examples: []string{"tool run\nnext"},
		arguments: []argumentSpec{{name: "value", valueType: "string", description: "argument"}},
		effective: []optionSpec{{name: "inherited", valueType: "bool", description: "option", origin: "parent"}},
	}
	var markdown strings.Builder
	writeMarkdownCommand(&markdown, markdownCommand, "tool", 2)
	for _, fragment := range []string{
		"summary", "description", "### Arguments", ": argument", "### Options",
		": option", "Inherited from `parent`.", "### Aliases", "### Examples",
	} {
		if !strings.Contains(markdown.String(), fragment) {
			t.Fatalf("Markdown missing %q: %q", fragment, markdown.String())
		}
	}
	var emptyMarkdown strings.Builder
	writeMarkdownCommand(&emptyMarkdown, &compiledCommand{name: "tool"}, "tool", 2)
	for _, section := range []string{"### Arguments", "### Options", "### Aliases", "### Examples"} {
		if strings.Contains(emptyMarkdown.String(), section) {
			t.Fatalf("empty Markdown contains %q: %q", section, emptyMarkdown.String())
		}
	}
	var localMarkdown strings.Builder
	writeMarkdownCommand(&localMarkdown, &compiledCommand{
		name:      "tool",
		options:   []optionSpec{{name: "local", valueType: "bool", origin: "tool"}},
		effective: []optionSpec{{name: "local", valueType: "bool", origin: "tool"}},
	}, "tool", 2)
	if !strings.Contains(localMarkdown.String(), "### Options") {
		t.Fatalf("local option Markdown = %q", localMarkdown.String())
	}
	for name, command := range map[string]*compiledCommand{
		"empty": {name: "tool", summary: "summary"},
		"equal": {name: "tool", summary: "summary", description: "summary"},
	} {
		var output strings.Builder
		writeMarkdownCommand(&output, command, "tool", 2)
		if strings.Count(output.String(), "summary") != 1 {
			t.Fatalf("Markdown(%s) duplicated description: %q", name, output.String())
		}
	}
}

func TestGenerationExactWrappingAndMetadataBoundaries(t *testing.T) {
	t.Parallel()

	if got := wrapHelp("abcd\n", 0); got != "abcd\n" {
		t.Fatalf("wrapHelp(width zero) = %q", got)
	}
	if got := wrapHelp("ab\n", 1); got != "a\nb\n" {
		t.Fatalf("wrapHelp(width one) = %q", got)
	}
	if got := wrapHelpLine("abcd", 4); len(got) != 1 || got[0] != "abcd" {
		t.Fatalf("wrapHelpLine(exact width) = %#v", got)
	}
	for _, test := range []struct {
		line  string
		width int
	}{
		{line: "-x  alpha beta", width: 8},
		{line: "  alpha beta", width: 6},
		{line: "-long  alpha", width: 6},
		{line: "word  alpha", width: 6},
		{line: "-abcdef", width: 3},
	} {
		lines := wrapHelpLine(test.line, test.width)
		if len(lines) < 2 {
			t.Fatalf("wrapHelpLine(%q, %d) = %#v", test.line, test.width, lines)
		}
		for _, line := range lines {
			if len([]rune(line)) > test.width {
				t.Fatalf("wrapped line %q exceeds width %d", line, test.width)
			}
		}
		if strings.HasPrefix(test.line, "-long") {
			for _, line := range lines[1:] {
				if !strings.HasPrefix(line, "     ") {
					t.Fatalf("option continuation indentation = %#v", lines)
				}
			}
		}
		if strings.HasPrefix(test.line, "word") {
			for _, line := range lines[1:] {
				if strings.HasPrefix(line, " ") {
					t.Fatalf("non-option continuation indentation = %#v", lines)
				}
			}
		}
	}

	if got := manifestOption(optionSpec{name: "plain"}).Short; got != "" {
		t.Fatalf("manifest zero short = %q", got)
	}
	if got := manifestOption(optionSpec{name: "short", short: 's'}).Short; got != "s" {
		t.Fatalf("manifest short = %q", got)
	}
	if got := commandPath(&compiledCommand{name: "child", effective: []optionSpec{{origin: "tool child"}}}); got != "tool child" {
		t.Fatalf("commandPath(suffix) = %q", got)
	}
	if got := commandPath(&compiledCommand{name: "child", effective: []optionSpec{{origin: "child"}}}); got != "child" {
		t.Fatalf("commandPath(exact) = %q", got)
	}
	application := &Application{root: &compiledCommand{name: "tool", children: []*compiledCommand{{
		name: "child", aliases: []string{"alias"},
	}}}}
	if command, canonical, err := application.findCommand([]string{"alias"}); err != nil || command.name != "child" || canonical != "tool child" {
		t.Fatalf("findCommand(alias) = %#v/%q/%v", command, canonical, err)
	}
}

type divergentOutputValue struct {
	json  string
	human string
}

func (value divergentOutputValue) MarshalJSON() ([]byte, error) { return json.Marshal(value.json) }
func (value divergentOutputValue) String() string               { return value.human }

type outputJSONOnly struct{}
type outputTextOnly struct{}
type outputStringOnly struct{}
type outputFormatterOnly struct{}
type outputErrorOnly struct{}
type outputPointerJSONOnly struct{}
type outputPointerTextOnly struct{}
type outputPointerStringOnly struct{}
type outputPointerFormatterOnly struct{}
type outputPointerErrorOnly struct{}
type recursiveOutputType struct{ Next *recursiveOutputType }
type outputJSONWithField struct{ Value int }

func (outputJSONOnly) MarshalJSON() ([]byte, error) { return []byte("null"), nil }
func (outputJSONWithField) MarshalJSON() ([]byte, error) {
	return []byte("null"), nil
}
func (outputTextOnly) MarshalText() ([]byte, error) { return nil, nil }
func (outputStringOnly) String() string             { return "" }
func (outputFormatterOnly) Format(fmt.State, rune)  {}
func (outputErrorOnly) Error() string               { return "" }

func (*outputPointerJSONOnly) MarshalJSON() ([]byte, error) { return []byte("null"), nil }
func (*outputPointerTextOnly) MarshalText() ([]byte, error) { return nil, nil }
func (*outputPointerStringOnly) String() string             { return "" }
func (*outputPointerFormatterOnly) Format(fmt.State, rune)  {}
func (*outputPointerErrorOnly) Error() string               { return "" }

func outputStructType(tag string) reflect.Type {
	return reflect.StructOf([]reflect.StructField{{
		Name: "DescriptiveField",
		Type: reflect.TypeFor[string](),
		Tag:  reflect.StructTag(tag),
	}})
}

func outputStructValue(tag string) any {
	typ := outputStructType(tag)
	value := reflect.New(typ).Elem()
	value.Field(0).SetString("x")

	return value.Interface()
}

func TestOutputAcceptsExactLimitsAndTracksCumulativeBytes(t *testing.T) {
	t.Parallel()

	records := &Output{infos: make([]string, maximumOutputRecords-1)}
	if err := records.Info(""); err != nil || len(records.infos) != maximumOutputRecords {
		t.Fatalf("Info(exact record limit) = %v, records = %d", err, len(records.infos))
	}
	if err := records.Info(""); !errors.Is(err, ErrOutput) {
		t.Fatalf("Info(over record limit) = %v", err)
	}

	bytesOutput := &Output{}
	if err := bytesOutput.Info("ab"); err != nil {
		t.Fatal(err)
	}
	if err := bytesOutput.Info(strings.Repeat("x", maximumOutputBytes-2)); err != nil {
		t.Fatalf("Info(exact byte limit) = %v", err)
	}
	if err := bytesOutput.Info("x"); !errors.Is(err, ErrOutput) {
		t.Fatalf("Info(over byte limit) = %v", err)
	}
	combined := &Output{bytes: 1, dataBytes: 1}
	if err := combined.Info(strings.Repeat("x", maximumOutputBytes-1)); !errors.Is(err, ErrOutput) {
		t.Fatalf("Info(combined byte overflow) = %v", err)
	}

	exactData := &Output{bytes: maximumOutputBytes - 3}
	if err := exactData.SetData("x"); err != nil {
		t.Fatalf("SetData(exact cumulative limit) = %v", err)
	}
	if err := (&Output{}).SetData(strings.Repeat("x", maximumOutputBytes-2)); err != nil {
		t.Fatalf("SetData(exact encoded limit) = %v", err)
	}
	for name, value := range map[string]divergentOutputValue{
		"encoded": {json: strings.Repeat("x", maximumOutputBytes), human: "small"},
		"human":   {json: "small", human: strings.Repeat("x", maximumOutputBytes+1)},
	} {
		if err := (&Output{}).SetData(value); !errors.Is(err, ErrOutput) {
			t.Fatalf("SetData(%s overflow) = %v", name, err)
		}
	}
}

func TestOutputPreflightBoundsAmplificationCyclesAndDepth(t *testing.T) {
	t.Parallel()

	for name, value := range map[string]any{
		"escaped expansion": strings.Repeat("&", maximumOutputBytes/6+1),
		"cycle": func() any {
			cycle := make(map[string]any)
			cycle["self"] = cycle
			return cycle
		}(),
		"depth": func() any {
			var value any = "bounded"
			for range maximumOutputDepth + 1 {
				next := value
				value = &next
			}
			return value
		}(),
	} {
		if err := (&Output{}).SetData(value); !errors.Is(err, ErrOutput) {
			t.Fatalf("SetData(%s) error = %v, want output classification", name, err)
		}
	}

	raw := json.RawMessage(`{"status":"ok"}`)
	if err := (&Output{}).SetData(raw); err != nil {
		t.Fatalf("SetData(json.RawMessage) error = %v", err)
	}
}

func TestOutputPreflightBoundsEncoderReachableTypeGraphs(t *testing.T) {
	t.Parallel()

	wrappers := map[string]func(reflect.Type) reflect.Type{
		"map":     func(typ reflect.Type) reflect.Type { return reflect.MapOf(reflect.TypeFor[string](), typ) },
		"pointer": reflect.PointerTo,
		"slice":   reflect.SliceOf,
		"struct": func(typ reflect.Type) reflect.Type {
			return reflect.StructOf([]reflect.StructField{{Name: "Value", Type: typ}})
		},
	}
	for name, wrap := range wrappers {
		typ := reflect.TypeFor[int]()
		for range maximumOutputDepth + 1 {
			typ = wrap(typ)
		}
		if name == "struct" {
			if err := newOutputTypeState().add(typ, 0); !errors.Is(err, ErrOutput) {
				t.Fatalf("outputTypeState.add(%s type depth) error = %v, want output classification", name, err)
			}
		}
		value := reflect.Zero(typ).Interface()
		if _, err := validateOutputValue(value); !errors.Is(err, ErrOutput) {
			t.Fatalf("validateOutputValue(%s type depth) error = %v, want output classification", name, err)
		}
	}

	if _, err := validateOutputValue((*recursiveOutputType)(nil)); err != nil {
		t.Fatalf("validateOutputValue(recursive type) error = %v", err)
	}

	countState := newOutputTypeState()
	countState.nodes = maximumOutputTypeNodes
	if err := countState.add(reflect.TypeFor[int](), 0); !errors.Is(err, ErrOutput) {
		t.Fatalf("outputTypeState.add(over count) error = %v, want output classification", err)
	}
	exactCountState := newOutputTypeState()
	exactCountState.nodes = maximumOutputTypeNodes - 1
	if err := exactCountState.add(reflect.TypeFor[int](), 0); err != nil {
		t.Fatalf("outputTypeState.add(exact count) error = %v", err)
	}
	fieldOverflowState := newOutputTypeState()
	fieldOverflowState.nodes = maximumOutputTypeNodes - 1
	if err := fieldOverflowState.add(reflect.TypeFor[struct{ Value int }](), 0); !errors.Is(err, ErrOutput) {
		t.Fatalf("outputTypeState.add(field count overflow) error = %v, want output classification", err)
	}
	exactFieldCountState := newOutputTypeState()
	exactFieldCountState.nodes = maximumOutputTypeNodes - 2
	exactFieldCountState.seen[reflect.TypeFor[int]()] = struct{}{}
	if err := exactFieldCountState.add(reflect.TypeFor[struct{ V int }](), 0); err != nil {
		t.Fatalf("outputTypeState.add(exact field count) error = %v", err)
	}
	metadataState := newOutputTypeState()
	metadataState.metadata = maximumOutputBytes
	if err := metadataState.add(outputStructType(`json:"\\invalid"`), 0); !errors.Is(err, ErrOutput) {
		t.Fatalf("outputTypeState.add(over metadata) error = %v, want output classification", err)
	}
	exactMetadataState := newOutputTypeState()
	exactMetadataState.metadata = maximumOutputBytes - 1
	if err := exactMetadataState.add(reflect.TypeFor[struct{ V int }](), 0); err != nil {
		t.Fatalf("outputTypeState.add(exact metadata) error = %v", err)
	}
	serializerState := newOutputTypeState()
	serializerState.metadata = maximumOutputBytes
	if err := serializerState.add(reflect.TypeFor[outputJSONWithField](), 0); err != nil {
		t.Fatalf("outputTypeState.add(JSON marshaler) error = %v", err)
	}
	if err := newOutputTypeState().add(nil, 0); err != nil {
		t.Fatalf("outputTypeState.add(nil) error = %v", err)
	}
	exactDepthType := reflect.TypeFor[int]()
	for range maximumOutputDepth {
		exactDepthType = reflect.PointerTo(exactDepthType)
	}
	if err := newOutputTypeState().add(exactDepthType, 0); err != nil {
		t.Fatalf("outputTypeState.add(exact depth) error = %v", err)
	}
}

func TestOutputPreflightMirrorsJSONTagFallbackAndBoundsMetadata(t *testing.T) {
	t.Parallel()

	for name, value := range map[string]any{
		"invalid name":     outputStructValue(`json:"\\invalid"`),
		"dash with option": outputStructValue(`json:"-,omitempty"`),
		"builtin omitzero": struct {
			Value int `json:",omitzero"`
		}{},
	} {
		size, err := estimateOutputSize(reflect.ValueOf(value), make(map[outputVisit]bool), 0)
		if err != nil {
			t.Fatalf("estimateOutputSize(%s tag) error = %v", name, err)
		}
		encoded, err := json.Marshal(value)
		if err != nil {
			t.Fatalf("json.Marshal(%s tag) error = %v", name, err)
		}
		if size.json < len(encoded) {
			t.Fatalf("estimateOutputSize(%s tag) JSON bytes = %d, encoded = %d", name, size.json, len(encoded))
		}
	}

	typ := reflect.StructOf([]reflect.StructField{{
		Name: "Value",
		Type: reflect.TypeFor[string](),
		Tag:  reflect.StructTag(strings.Repeat("x", maximumOutputBytes+1)),
	}})
	if _, err := validateOutputValue(reflect.Zero(typ).Interface()); !errors.Is(err, ErrOutput) {
		t.Fatalf("validateOutputValue(oversize struct tag) error = %v, want output classification", err)
	}
}

func TestOutputPreflightBoundsRawMessageEncodingExpansion(t *testing.T) {
	t.Parallel()

	raw := json.RawMessage(`"` + strings.Repeat("&", maximumOutputBytes/6+1) + `"`)
	if _, err := validateOutputValue(raw); !errors.Is(err, ErrOutput) {
		t.Fatalf("validateOutputValue(expanding raw message) error = %v, want output classification", err)
	}
}

func TestOutputPreflightBoundsCollectionCount(t *testing.T) {
	t.Parallel()

	if err := (&Output{}).SetData(make([]struct{}, maximumOutputElements)); err != nil {
		t.Fatalf("SetData(exact collection limit) error = %v", err)
	}
	if err := (&Output{}).SetData(make([]struct{}, maximumOutputElements+1)); !errors.Is(err, ErrOutput) {
		t.Fatalf("SetData(over collection limit) error = %v, want output classification", err)
	}
	exactMap := make(map[int]struct{}, maximumOutputElements)
	for index := range maximumOutputElements {
		exactMap[index] = struct{}{}
	}
	if _, err := estimateOutputSize(
		reflect.ValueOf(exactMap), make(map[outputVisit]bool), 0,
	); err != nil {
		t.Fatalf("estimateOutputSize(exact map collection limit) error = %v", err)
	}
	exactMap[maximumOutputElements] = struct{}{}
	if _, err := estimateOutputSize(
		reflect.ValueOf(exactMap), make(map[outputVisit]bool), 0,
	); !errors.Is(err, ErrOutput) {
		t.Fatalf("estimateOutputSize(over map collection limit) error = %v, want output classification", err)
	}
}

func TestOutputPreflightAccountsForBuiltInSerializationShapes(t *testing.T) {
	t.Parallel()

	number := 42
	accepted := []any{
		nil,
		time.Second,
		(*int)(nil),
		&number,
		true,
		int64(-42),
		uint64(42),
		float64(42),
		[2]string{"first", "second"},
		[]string(nil),
		[]string{"first", "second"},
		map[string]int(nil),
		map[string]int{"first": 1, "second": 2},
		map[int]string{-1: "signed"},
		map[uint]string{1: "unsigned"},
		struct {
			hidden  string
			Visible int    `json:",string"`
			Ignored string `json:"-"`
			Named   string `json:"named"`
		}{hidden: "private", Visible: 42, Ignored: "ignored", Named: "public"},
		"quote \" slash \\ control \x01 invalid " + string([]byte{0xff}),
	}
	for index, value := range accepted {
		if err := (&Output{}).SetData(value); err != nil {
			t.Fatalf("SetData(accepted[%d]) error = %v", index, err)
		}
	}

	var nilInterface any
	if _, err := estimateOutputSize(
		reflect.ValueOf(&nilInterface).Elem(), make(map[outputVisit]bool), 0,
	); err != nil {
		t.Fatalf("estimateOutputSize(nil interface) error = %v", err)
	}

	type node struct{ Next *node }
	pointerCycle := new(node)
	pointerCycle.Next = pointerCycle
	sliceCycle := make([]any, 1)
	sliceCycle[0] = sliceCycle
	large := strings.Repeat("x", maximumOutputBytes)
	rejected := []any{
		pointerCycle,
		sliceCycle,
		[]any{divergentOutputValue{}},
		append([]string{large}, "overflow"),
		map[divergentOutputValue]int{{}: 1},
		map[string]any{"value": divergentOutputValue{}},
		map[string]string{"value": large},
		map[bool]string{true: "unsupported"},
		struct{ Value divergentOutputValue }{},
		struct{ Value string }{Value: large},
		json.RawMessage(strings.Repeat(" ", maximumOutputBytes/4+1)),
		make(chan int),
	}
	for index, value := range rejected {
		if err := (&Output{}).SetData(value); !errors.Is(err, ErrOutput) {
			t.Fatalf("SetData(rejected[%d]) error = %v, want output classification", index, err)
		}
	}

	invalidBytes := strings.Repeat(string([]byte{0xff}), maximumOutputBytes/3+1)
	if err := (&Output{}).Info(invalidBytes); !errors.Is(err, ErrOutput) {
		t.Fatalf("Info(invalid UTF-8 amplification) error = %v, want output classification", err)
	}
	controlBytes := strings.Repeat("\x01", maximumOutputBytes+1)
	if err := (&Output{}).Info(controlBytes); !errors.Is(err, ErrOutput) {
		t.Fatalf("Info(control-only input) error = %v, want output classification", err)
	}
	if err := (&Output{}).SetData(struct{ hidden string }{hidden: invalidBytes}); !errors.Is(err, ErrOutput) {
		t.Fatalf("SetData(hidden invalid UTF-8) error = %v, want output classification", err)
	}
}

func TestOutputSizeEstimationExactContracts(t *testing.T) {
	t.Parallel()

	assertSize := func(value any, want outputSize) {
		t.Helper()
		got, err := estimateOutputSize(
			reflect.ValueOf(value), make(map[outputVisit]bool), 0,
		)
		if err != nil || got != want {
			t.Fatalf("estimateOutputSize(%T) = %#v, %v; want %#v", value, got, err, want)
		}
	}

	assertSize(nil, outputSize{json: 4, human: 5})
	assertSize(time.Second, outputSize{json: 21, human: 32})
	assertSize(json.RawMessage("null"), outputSize{json: 4, human: 18})
	assertSize((*int)(nil), outputSize{json: 4, human: 5})
	number := 42
	assertSize(&number, outputSize{json: 2, human: 2 + strconv.IntSize/4})
	assertSize("a", outputSize{json: 3, human: 1})
	assertSize(true, outputSize{json: 5, human: 5})
	assertSize(int64(-42), outputSize{json: 3, human: 3})
	assertSize(uint64(42), outputSize{json: 2, human: 2})
	assertSize(float64(42), outputSize{json: 32, human: 32})
	assertSize([1]string{"a"}, outputSize{json: 5, human: 3})
	assertSize([2]string{"a", "bb"}, outputSize{json: 10, human: 6})
	assertSize([3]string{"a", "bb", "ccc"}, outputSize{json: 16, human: 10})
	assertSize([]string(nil), outputSize{json: 4, human: 2})
	assertSize([]string{"a"}, outputSize{json: 5, human: 3})
	assertSize([]string{"a", "bb"}, outputSize{json: 10, human: 6})
	assertSize([]string{"a", "bb", "ccc"}, outputSize{json: 16, human: 10})
	assertSize(map[string]int(nil), outputSize{json: 4, human: 5})
	assertSize(map[string]int{"a": 1}, outputSize{json: 7, human: 8})
	assertSize(map[string]int{"a": 1, "bb": 22}, outputSize{json: 15, human: 14})
	assertSize(map[string]int{"a": 1, "bb": 22, "ccc": 333}, outputSize{json: 25, human: 22})
	assertSize(struct {
		hidden  string
		Visible int    `json:",string"`
		Ignored string `json:"-"`
		Named   string `json:"named"`
	}{hidden: "private", Visible: 42, Ignored: "ignored", Named: "public"},
		outputSize{json: 43, human: 27})
	assertSize(make(chan int), outputSize{json: 64, human: 64})

	var nilInterface any
	got, err := estimateOutputSize(
		reflect.ValueOf(&nilInterface).Elem(), make(map[outputVisit]bool), 0,
	)
	if err != nil || got != (outputSize{json: 4, human: 5}) {
		t.Fatalf("estimateOutputSize(nil interface) = %#v, %v", got, err)
	}
	if _, err := estimateOutputSize(
		reflect.ValueOf("depth"), make(map[outputVisit]bool), maximumOutputDepth,
	); err != nil {
		t.Fatalf("estimateOutputSize(exact depth) error = %v", err)
	}
	if _, err := estimateOutputSize(
		reflect.ValueOf("depth"), make(map[outputVisit]bool), maximumOutputDepth+1,
	); !errors.Is(err, ErrOutput) {
		t.Fatalf("estimateOutputSize(over depth) error = %v", err)
	}

	for name, value := range map[string]any{
		"json value":        outputJSONOnly{},
		"text value":        outputTextOnly{},
		"string value":      outputStringOnly{},
		"formatter value":   outputFormatterOnly{},
		"error value":       outputErrorOnly{},
		"json pointer":      outputPointerJSONOnly{},
		"text pointer":      outputPointerTextOnly{},
		"string pointer":    outputPointerStringOnly{},
		"formatter pointer": outputPointerFormatterOnly{},
		"error pointer":     outputPointerErrorOnly{},
	} {
		if !hasCustomSerialization(reflect.TypeOf(value)) {
			t.Fatalf("hasCustomSerialization(%s) = false", name)
		}
	}
	if hasCustomSerialization(reflect.TypeOf(struct{}{})) ||
		hasCustomSerialization(reflect.TypeOf(new(struct{}))) {
		t.Fatal("plain values report custom serialization")
	}

	for _, test := range []struct {
		value any
		want  outputSize
	}{
		{value: int64(-42), want: outputSize{json: 5, human: 3}},
		{value: uint64(42), want: outputSize{json: 4, human: 2}},
	} {
		got, keyErr := estimateMapKeySize(
			reflect.ValueOf(test.value), make(map[outputVisit]bool), 0,
		)
		if keyErr != nil || got != test.want {
			t.Fatalf("estimateMapKeySize(%T) = %#v, %v; want %#v", test.value, got, keyErr, test.want)
		}
	}
	if _, err := estimateMapKeySize(
		reflect.ValueOf(true), make(map[outputVisit]bool), 0,
	); !errors.Is(err, ErrOutput) {
		t.Fatalf("estimateMapKeySize(bool) error = %v", err)
	}
}

func TestOutputSizeHelpersExactBoundaries(t *testing.T) {
	t.Parallel()

	for value, want := range map[string]int{
		"":                   2,
		"a":                  3,
		" ":                  3,
		"\\":                 4,
		"\"":                 4,
		"\x01":               8,
		"<":                  8,
		">":                  8,
		"&":                  8,
		"\u2028":             8,
		"\u2029":             8,
		"é":                  4,
		string([]byte{0xff}): 8,
	} {
		if got := jsonStringSize(value); got != want {
			t.Fatalf("jsonStringSize(%q) = %d, want %d", value, got, want)
		}
	}
	if got := jsonStringSize(strings.Repeat("a", maximumOutputBytes-2)); got != maximumOutputBytes {
		t.Fatalf("jsonStringSize(exact) = %d", got)
	}
	if got := jsonStringSize(strings.Repeat("a", maximumOutputBytes-1)); got != maximumOutputBytes+1 {
		t.Fatalf("jsonStringSize(over) = %d", got)
	}

	for value, want := range map[string]int{
		"":                   0,
		"a":                  1,
		"\x01":               0,
		"é":                  2,
		string([]byte{0xff}): 3,
	} {
		if got := humanStringSize(value); got != want {
			t.Fatalf("humanStringSize(%q) = %d, want %d", value, got, want)
		}
	}
	if got := humanStringSize(strings.Repeat("a", maximumOutputBytes)); got != maximumOutputBytes {
		t.Fatalf("humanStringSize(exact) = %d", got)
	}
	if got := humanStringSize(strings.Repeat("a", maximumOutputBytes+1)); got != maximumOutputBytes+1 {
		t.Fatalf("humanStringSize(over) = %d", got)
	}
	if got := humanStringSize(strings.Repeat("\x01", maximumOutputBytes+2)); got != maximumOutputBytes+1 {
		t.Fatalf("humanStringSize(control-only over) = %d", got)
	}
	if got := rawMessageJSONSize(bytes.Repeat([]byte{'a'}, maximumOutputBytes+2)); got != maximumOutputBytes+1 {
		t.Fatalf("rawMessageJSONSize(over) = %d", got)
	}
	if got := rawMessageJSONSize([]byte{0xe2, 0x80, 0xa8}); got != 6 {
		t.Fatalf("rawMessageJSONSize(U+2028) = %d, want 6", got)
	}
	for name, raw := range map[string][]byte{
		"truncated lead":         {0xe2},
		"truncated continuation": {0xe2, 0x80},
		"wrong continuation":     {0xe2, 0x81, 0xa8},
		"wrong trailing byte":    {0xe2, 0x80, 0xaa},
	} {
		if got := rawMessageJSONSize(raw); got != len(raw) {
			t.Fatalf("rawMessageJSONSize(%s) = %d, want %d", name, got, len(raw))
		}
	}
	exactRaw := bytes.Repeat([]byte{'a'}, maximumOutputBytes)
	if got := rawMessageJSONSize(exactRaw); got != maximumOutputBytes {
		t.Fatalf("rawMessageJSONSize(exact) = %d", got)
	}
	expandingRaw := append(bytes.Repeat([]byte{'a'}, maximumOutputBytes-13), '&', '&', '&')
	if got := rawMessageJSONSize(expandingRaw); got != maximumOutputBytes+1 {
		t.Fatalf("rawMessageJSONSize(late expansion) = %d", got)
	}
	if hasCustomZeroCheck(reflect.TypeFor[*int]()) {
		t.Fatal("pointer type unexpectedly has a custom zero check")
	}
	if !jsonTagOption("omitempty,string", "string") {
		t.Fatal("jsonTagOption() did not inspect the second option")
	}
	for name, want := range map[string]bool{
		"":   false,
		"a":  true,
		"9":  true,
		"\\": false,
	} {
		if got := validJSONTagName(name); got != want {
			t.Fatalf("validJSONTagName(%q) = %t, want %t", name, got, want)
		}
	}
	humanExpansion := string(append(bytes.Repeat([]byte{'a'}, maximumOutputBytes-3), 0xff, 0xff))
	if got := humanStringSize(humanExpansion); got != maximumOutputBytes+1 {
		t.Fatalf("humanStringSize(late expansion) = %d", got)
	}

	if got := boundedSum(maximumOutputBytes, 0); got != maximumOutputBytes {
		t.Fatalf("boundedSum(exact) = %d", got)
	}
	if got := boundedSum(maximumOutputBytes, 1); got != maximumOutputBytes+1 {
		t.Fatalf("boundedSum(over) = %d", got)
	}
	if got := boundedSum(maximumOutputBytes-1, 3); got != maximumOutputBytes+1 {
		t.Fatalf("boundedSum(clamped overflow) = %d", got)
	}
	for _, test := range []struct {
		left  int
		right int
		want  int
	}{
		{left: 0, right: maximumOutputBytes + 1, want: 0},
		{left: maximumOutputBytes, right: 1, want: maximumOutputBytes},
		{left: maximumOutputBytes, right: 2, want: maximumOutputBytes + 1},
		{left: 2, right: 3, want: 6},
	} {
		if got := boundedProduct(test.left, test.right); got != test.want {
			t.Fatalf("boundedProduct(%d, %d) = %d, want %d", test.left, test.right, got, test.want)
		}
	}
	if outputChildDepth(0) != 1 || outputChildDepth(maximumOutputDepth) != maximumOutputDepth+1 {
		t.Fatalf("outputChildDepth boundaries = %d/%d", outputChildDepth(0), outputChildDepth(maximumOutputDepth))
	}
	if outputSeparatorSize(0) != 0 || outputSeparatorSize(1) != 1 || outputSeparatorSize(2) != 1 {
		t.Fatalf("outputSeparatorSize(0/1/2) = %d/%d/%d", outputSeparatorSize(0), outputSeparatorSize(1), outputSeparatorSize(2))
	}
	for _, size := range []outputSize{
		{json: maximumOutputBytes + 1},
		{human: maximumOutputBytes + 1},
		{json: maximumOutputBytes + 1, human: maximumOutputBytes + 1},
	} {
		if !outputSizeExceeded(size) {
			t.Fatalf("outputSizeExceeded(%#v) = false", size)
		}
	}
	for _, size := range []outputSize{
		{},
		{json: maximumOutputBytes},
		{human: maximumOutputBytes},
		{json: maximumOutputBytes, human: maximumOutputBytes},
	} {
		if outputSizeExceeded(size) {
			t.Fatalf("outputSizeExceeded(%#v) = true", size)
		}
	}

	if size, err := validateOutputValue(strings.Repeat("a", maximumOutputBytes-2)); err != nil || size.json != maximumOutputBytes {
		t.Fatalf("validateOutputValue(exact JSON) = %#v, %v", size, err)
	}
	if _, err := validateOutputValue(strings.Repeat("a", maximumOutputBytes-1)); !errors.Is(err, ErrOutput) {
		t.Fatalf("validateOutputValue(over JSON) error = %v", err)
	}
	invalid := strings.Repeat(string([]byte{0xff}), (maximumOutputBytes-2)/3) + "xx"
	if size, err := validateOutputValue(struct{ hidden string }{hidden: invalid}); err != nil || size.human != maximumOutputBytes {
		t.Fatalf("validateOutputValue(exact human) = %#v, %v", size, err)
	}
	if _, err := validateOutputValue(struct{ hidden string }{hidden: invalid + "x"}); !errors.Is(err, ErrOutput) {
		t.Fatalf("validateOutputValue(over human) error = %v", err)
	}
}

func TestProtectedErrorAndSecretClassificationEdgeCases(t *testing.T) {
	t.Parallel()

	if err := newProtectedError(ErrorKindOutput, "safe", nil); !errors.Is(err, ErrOutput) {
		t.Fatalf("newProtectedError(nil cause) = %v, want output classification", err)
	}
	if commandProtectsSecrets(nil) {
		t.Fatal("nil command protects secrets")
	}
	var nilCause *protectedCause
	if nilCause.Is(errors.New("target")) {
		t.Fatal("nil protected cause matched a target")
	}
	classified := newClassifiedError(ErrorKindOutput, "safe", nil, false)
	err := classifyPhaseError(
		context.Background(),
		&compiledCommand{effective: []optionSpec{{secret: true}}},
		"callback failed",
		classified,
	)
	var result *Error
	if !errors.As(err, &result) || result.Kind() != ErrorKindOutput {
		t.Fatalf("classifyPhaseError() = %v, want protected output classification", err)
	}
}

func TestRunPreflightSeparatesApplicationModeAndContextFailures(t *testing.T) {
	t.Parallel()

	for name, application := range map[string]*Application{
		"nil":   nil,
		"empty": {},
	} {
		for _, run := range []func(context.Context, Request) Result{
			application.Run,
			application.RunCommand,
		} {
			if result := run(context.Background(), Request{}); !errors.Is(result.Err, ErrInternal) {
				t.Fatalf("%s application result = %#v", name, result)
			}
		}
	}

	called := 0
	application, err := Compile(NewCommand("tool", WithHandler(func(context.Context, Invocation) error {
		called++

		return nil
	})))
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []OutputMode{OutputHuman, OutputJSON, OutputQuiet} {
		if result := application.RunCommand(context.Background(), Request{Output: OutputPolicy{Mode: mode}}); result.Err != nil {
			t.Fatalf("RunCommand(mode %d) = %#v", mode, result)
		}
	}
	if called != 3 {
		t.Fatalf("valid mode handler calls = %d", called)
	}
	if result := application.RunCommand(context.Background(), Request{
		Output: OutputPolicy{Mode: OutputMode(255)},
	}); !errors.Is(result.Err, ErrInternal) {
		t.Fatalf("RunCommand(invalid mode) = %#v", result)
	}

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	for _, run := range []func(context.Context, Request) Result{application.Run, application.RunCommand} {
		if result := run(canceled, Request{}); !errors.Is(result.Err, context.Canceled) {
			t.Fatalf("canceled run = %#v", result)
		}
	}
	if called != 3 {
		t.Fatalf("canceled handler calls = %d", called)
	}
}

func TestCleanupUsesTheDocumentedDefaultDeadline(t *testing.T) {
	t.Parallel()

	var remaining time.Duration
	command := &compiledCommand{cleanup: []Handler{func(ctx context.Context, _ Invocation) error {
		deadline, ok := ctx.Deadline()
		if !ok {
			t.Fatal("cleanup context has no deadline")
		}
		remaining = time.Until(deadline)

		return nil
	}}}
	if err := executeCleanup(context.Background(), command, Invocation{}); err != nil {
		t.Fatalf("executeCleanup() error = %v", err)
	}
	if remaining < 29*time.Second || remaining > 30*time.Second {
		t.Fatalf("cleanup deadline remaining = %v", remaining)
	}
}

func TestCleanupWaitsForCooperativeDeadlineHandling(t *testing.T) {
	t.Parallel()

	started := time.Now()
	command := &compiledCommand{cleanup: []Handler{func(ctx context.Context, _ Invocation) error {
		<-ctx.Done()
		return ctx.Err()
	}}}
	err := executeCleanupWithin(context.Background(), command, Invocation{}, 10*time.Millisecond)
	if !errors.Is(err, ErrCleanup) || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("executeCleanupWithin() error = %v, want cleanup deadline", err)
	}
	if elapsed := time.Since(started); elapsed < 5*time.Millisecond || elapsed > time.Second {
		t.Fatalf("cooperative cleanup elapsed = %v, want deadline-bounded return", elapsed)
	}
}

func TestCompletionProtocolDescriptionAndContextMatrix(t *testing.T) {
	t.Parallel()

	application := &Application{
		root: &compiledCommand{name: "tool", children: []*compiledCommand{
			{name: "described", summary: "description"},
			{name: "plain"},
		}},
		limits: defaultLimits(),
	}
	for _, test := range []struct {
		name                string
		withoutDescriptions bool
		want                string
	}{
		{name: "descriptions", want: "described\tdescription\nplain\n:4\n"},
		{name: "without descriptions", withoutDescriptions: true, want: "described\nplain\n:4\n"},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			var stdout bytes.Buffer
			result := application.runCompletionBoundary(
				context.Background(),
				[]string{""},
				test.withoutDescriptions,
				IO{Stdout: &stdout},
			)
			if result.Err != nil || stdout.String() != test.want {
				t.Fatalf("runCompletionBoundary() = %#v, output = %q", result, stdout.String())
			}
		})
	}
	var protocol bytes.Buffer
	result := application.Run(context.Background(), Request{
		Args: []string{"__completeNoDesc", ""}, Stdout: &protocol,
	})
	if result.Err != nil || protocol.String() != "described\nplain\n:4\n" {
		t.Fatalf("Run(__completeNoDesc) = %#v, output = %q", result, protocol.String())
	}
	for name, makeContext := range map[string]func() context.Context{
		"canceled": func() context.Context {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			return ctx
		},
		"deadline": func() context.Context {
			ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
			defer cancel()

			return ctx
		},
	} {
		var stdout bytes.Buffer
		result := application.runCompletionBoundary(makeContext(), nil, false, IO{Stdout: &stdout})
		var classified *Error
		if !errors.As(result.Err, &classified) ||
			name == "canceled" && classified.Kind() != ErrorKindCanceled ||
			name == "deadline" && classified.Kind() != ErrorKindDeadline ||
			stdout.String() != ":5\n" {
			t.Fatalf("%s completion = %#v, output = %q", name, result, stdout.String())
		}
	}
}

func TestInvocationInteractiveStateMatrix(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name           string
		interaction    Interaction
		nonInteractive bool
		want           bool
		wantUsage      bool
	}{
		{name: "optional", interaction: InteractionOptional, want: true},
		{name: "optional non-interactive", interaction: InteractionOptional, nonInteractive: true},
		{name: "required", interaction: InteractionRequired, want: true},
		{name: "required non-interactive", interaction: InteractionRequired, nonInteractive: true, wantUsage: true},
		{name: "forbidden", interaction: InteractionForbidden},
		{name: "forbidden non-interactive", interaction: InteractionForbidden, nonInteractive: true},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			called := false
			application, err := Compile(NewCommand(
				"tool",
				WithInteraction(test.interaction),
				WithHandler(func(_ context.Context, invocation Invocation) error {
					called = true
					if invocation.Interactive() != test.want {
						t.Fatalf("Interactive() = %v, want %v", invocation.Interactive(), test.want)
					}

					return nil
				}),
			))
			if err != nil {
				t.Fatal(err)
			}
			result := application.RunCommand(context.Background(), Request{NonInteractive: test.nonInteractive})
			if test.wantUsage {
				if !errors.Is(result.Err, ErrUsage) || called {
					t.Fatalf("RunCommand() = %#v, called = %v", result, called)
				}
			} else if result.Err != nil || !called {
				t.Fatalf("RunCommand() = %#v, called = %v", result, called)
			}
		})
	}
}

func TestSuggestionBoundsAndTieBreaking(t *testing.T) {
	t.Parallel()

	if got := suggestCommand(&compiledCommand{children: []*compiledCommand{{name: "bat"}, {name: "car"}}}, "cat"); got != "bat" {
		t.Fatalf("tie suggestion = %q", got)
	}
	if got := suggestCommand(&compiledCommand{children: []*compiledCommand{{name: "bat"}}}, "cat"); got != "bat" {
		t.Fatalf("threshold suggestion = %q", got)
	}
	if got := suggestCommand(&compiledCommand{children: []*compiledCommand{{name: "bots"}}}, "cats"); got != "bots" {
		t.Fatalf("four-rune threshold suggestion = %q", got)
	}
	long64 := strings.Repeat("x", 64)
	long65 := strings.Repeat("x", 65)
	if got := suggestCommand(&compiledCommand{children: []*compiledCommand{{name: long64}}}, long64); got != long64 {
		t.Fatalf("64-rune suggestion = %q", got)
	}
	if got := suggestCommand(&compiledCommand{children: []*compiledCommand{{name: long65}}}, long65); got != "" {
		t.Fatalf("65-rune suggestion = %q", got)
	}
	if got := suggestCommand(&compiledCommand{children: []*compiledCommand{{name: long65}, {name: "target"}}}, "target"); got != "target" {
		t.Fatalf("oversized candidate stopped traversal, suggestion = %q", got)
	}

	children := make([]*compiledCommand, 0, 101)
	for index := range 100 {
		children = append(children, &compiledCommand{name: "candidate" + string(rune('Ā'+index))})
	}
	children = append(children, &compiledCommand{name: "target"})
	if got := suggestCommand(&compiledCommand{children: children}, "target"); got != "" {
		t.Fatalf("candidate beyond bound = %q", got)
	}
	for index := range 100 {
		children[index].hidden = true
	}
	if got := suggestCommand(&compiledCommand{children: children}, "target"); got != "target" {
		t.Fatalf("hidden candidates consumed bound, suggestion = %q", got)
	}
}

func TestBoundedEditDistanceMatchesReferenceAtEverySmallBoundary(t *testing.T) {
	t.Parallel()

	values := []string{"", "a", "b", "aa", "ab", "ba", "bb", "aaa", "aba", "bbb"}
	for _, left := range values {
		for _, right := range values {
			wantDistance := referenceEditDistance([]rune(left), []rune(right))
			for limit := range 4 {
				want := wantDistance
				if want > limit {
					want = limit + 1
				}
				if got := boundedEditDistance([]rune(left), []rune(right), limit); got != want {
					t.Fatalf("boundedEditDistance(%q, %q, %d) = %d, want %d", left, right, limit, got, want)
				}
			}
		}
	}
}

func referenceEditDistance(left, right []rune) int {
	previous := make([]int, len(right)+1)
	current := make([]int, len(right)+1)
	for index := range previous {
		previous[index] = index
	}
	for leftIndex, leftRune := range left {
		current[0] = leftIndex + 1
		for rightIndex, rightRune := range right {
			cost := 1
			if leftRune == rightRune {
				cost = 0
			}
			current[rightIndex+1] = min(
				min(current[rightIndex]+1, previous[rightIndex+1]+1),
				previous[rightIndex]+cost,
			)
		}
		previous, current = current, previous
	}

	return previous[len(right)]
}

func TestRuntimeOptionGroupsAndArgvAcceptExactBounds(t *testing.T) {
	t.Parallel()

	a, b := new(int), new(int)
	for _, test := range []struct {
		name   string
		group  optionGroupSpec
		values map[any]resolvedValue
		want   bool
	}{
		{name: "exclusive empty", group: optionGroupSpec{kind: optionGroupExclusive, bindings: []any{a, b}}},
		{name: "exclusive one", group: optionGroupSpec{kind: optionGroupExclusive, bindings: []any{a, b}}, values: map[any]resolvedValue{a: {state: ValueExplicit}}},
		{name: "exclusive two", group: optionGroupSpec{kind: optionGroupExclusive, bindings: []any{a, b}}, values: map[any]resolvedValue{a: {state: ValueExplicit}, b: {state: ValueExplicit}}, want: true},
		{name: "together empty", group: optionGroupSpec{kind: optionGroupTogether, bindings: []any{a, b}}},
		{name: "together partial", group: optionGroupSpec{kind: optionGroupTogether, bindings: []any{a, b}}, values: map[any]resolvedValue{a: {state: ValueExplicit}}, want: true},
		{name: "together all", group: optionGroupSpec{kind: optionGroupTogether, bindings: []any{a, b}}, values: map[any]resolvedValue{a: {state: ValueExplicit}, b: {state: ValueExplicit}}},
	} {
		err := validateOptionGroups([]optionGroupSpec{test.group}, test.values)
		if errors.Is(err, ErrUsage) != test.want {
			t.Fatalf("%s error = %v, want usage = %v", test.name, err, test.want)
		}
	}

	limits := Limits{MaximumArguments: 2, MaximumArgvBytes: 5}
	if err := validateArgv([]string{"ab", "cde"}, limits); err != nil {
		t.Fatalf("validateArgv(exact limits) error = %v", err)
	}
	if err := validateArgv([]string{"a", "b", "c"}, limits); !errors.Is(err, ErrUsage) {
		t.Fatalf("validateArgv(argument overflow) error = %v", err)
	}
	if err := validateArgv([]string{"ab", "cdef"}, limits); !errors.Is(err, ErrUsage) {
		t.Fatalf("validateArgv(byte overflow) error = %v", err)
	}
}

func TestFailureResultAndContextCausePreserveExactState(t *testing.T) {
	t.Parallel()

	failure := errors.New("failure")
	if result := failureResult(nil, failure); result.Command.command != nil || !errors.Is(result.Err, failure) {
		t.Fatalf("failureResult(nil) = %#v", result)
	}
	command := &compiledCommand{name: "tool"}
	if result := failureResult(command, failure); result.Command.command != command || !errors.Is(result.Err, failure) {
		t.Fatalf("failureResult(command) = %#v", result)
	}
	cause := errors.New("cancel cause")
	ctx, cancel := context.WithCancelCause(context.Background())
	cancel(cause)
	if err := contextError(ctx); !errors.Is(err, cause) || !errors.Is(err, ErrCanceled) {
		t.Fatalf("contextError(cause) = %v", err)
	}
}
