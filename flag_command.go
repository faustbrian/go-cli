package cli

import (
	"context"
	"errors"
	"flag"
	"io"

	"github.com/faustbrian/go-cli/internal/engine"
)

// CompileFlagCommand compiles a flat typed command using Go's flag.FlagSet
// token, ordering and help rules instead of the normal command grammar. Only
// root options are supported: children, positional definitions and shorthands
// are rejected with ErrInternal. One- and two-dash names are equivalent;
// parsing stops at the first positional, which is rejected. Typed conversion
// occurs once per occurrence, before later flags or help, and scalar values
// retain the last successful occurrence. Unknown/missing flags are ErrUsage;
// typed conversion retains ErrMalformedValue and secret protection. Normal
// argv limits, defaults, validation, lifecycle, IO and generated help remain
// owned by this package. Run and RunCommand do not dispatch hidden completion
// tokens or implicit version flags. No global flag set or process streams are used.
func CompileFlagCommand(root *Command, options ...CompileOption) (*Application, error) {
	application, err := Compile(root, options...)
	if err != nil {
		return nil, err
	}
	if len(application.root.children) != 0 || len(application.root.arguments) != 0 {
		return nil, newInternalError("flag command requires root options without children or positionals", nil)
	}
	for _, option := range application.root.options {
		if option.short != 0 {
			return nil, newInternalError("flag command does not support shorthands", nil)
		}
	}
	application.flagSetParsing = true
	return application, nil
}

// parseInput delegates ordinary graphs to the command engine and flat legacy
// invocations to an invocation-local native flag set.
func (application *Application) parseInput(ctx context.Context, argv []string) (engine.Result, map[int]any, error) {
	if !application.flagSetParsing {
		parsed, err := engine.Parse(ctx, engineCommand(application.root), argv)
		return parsed, nil, err
	}
	flags := flag.NewFlagSet(application.root.name, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	parsed := engine.Result{CommandID: application.root.id, Options: make(map[int][]string)}
	converted := make(map[int]any)
	var conversionError error
	for _, option := range application.root.options {
		flags.Var(&flagCommandValue{option: option, parsed: &parsed, converted: converted, failure: &conversionError}, option.name, "")
	}
	if err := flags.Parse(argv); err != nil {
		if conversionError != nil {
			return engine.Result{}, nil, conversionError
		}
		if errors.Is(err, flag.ErrHelp) {
			parsed.Action = engine.ActionHelp
			return parsed, converted, nil
		}
		// Native diagnostics may contain hostile argv. Never retain or forward
		// them; this surface exposes a categorical usage failure instead.
		return engine.Result{}, nil, &engine.ParseError{Kind: engine.FailureUsage}
	}
	if flags.NArg() != 0 {
		return engine.Result{}, nil, &engine.ParseError{Kind: engine.FailureUsage}
	}
	return parsed, converted, nil
}

// flagCommandValue belongs to one invocation and converts before flag parsing
// advances. Converted values are reused rather than invoking parsers twice.
type flagCommandValue struct {
	option    optionSpec
	parsed    *engine.Result
	converted map[int]any
	failure   *error
}

// String deliberately withholds defaults and supplied values from native flag
// diagnostics, which are discarded rather than used as public CLI output.
func (*flagCommandValue) String() string { return "" }

// IsBoolFlag keeps the native bare-boolean and assigned-value rules.
func (value *flagCommandValue) IsBoolFlag() bool { return value.option.boolean }

// Set retains an occurrence only after its typed conversion succeeds.
func (value *flagCommandValue) Set(raw string) error {
	values := value.parsed.Options[value.option.key]
	values = append(values, raw)
	converted, err := value.option.parse(values)
	if err != nil {
		*value.failure = invalidValue("option --"+value.option.name, value.option.secret, err)
		return *value.failure
	}
	value.parsed.Options[value.option.key] = values
	value.converted[value.option.key] = converted
	return nil
}
