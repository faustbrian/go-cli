package benchmarks

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/alecthomas/kong"
	framework "github.com/faustbrian/go-cli"
	"github.com/spf13/cobra"
	urfave "github.com/urfave/cli/v3"
)

var comparisonArgv = []string{"tool", "deploy", "--force", "target"}

var errComparisonInput = errors.New("invalid input")

type comparisonClassifiedError struct {
	kind    error
	message string
	cause   error
}

func (failure *comparisonClassifiedError) Error() string { return failure.message }
func (failure *comparisonClassifiedError) Unwrap() error { return failure.cause }
func (failure *comparisonClassifiedError) Is(target error) bool {
	return target == failure.kind
}

var errComparisonUsage = &comparisonClassifiedError{
	kind: framework.ErrUsage, message: "missing required argument target",
}

var errComparisonValidation = &comparisonClassifiedError{
	kind: framework.ErrValidation, message: "command validation failed: invalid input",
	cause: errComparisonInput,
}

const comparisonMaximumOutputBytes = 1 << 20

type comparisonResult struct {
	Target string `json:"target"`
	Force  bool   `json:"force"`
}

func validateComparison(force bool, target string, arguments int) error {
	if arguments != 1 {
		return errComparisonUsage
	}
	if !force || target == "" {
		return errComparisonInput
	}

	return nil
}

func comparisonResultForTarget(target string, force bool) comparisonResult {
	if target == "oversize" {
		target = strings.Repeat("x", comparisonMaximumOutputBytes)
	}

	return comparisonResult{Target: target, Force: force}
}

type comparisonOutput struct {
	mu        sync.Mutex
	dataJSON  json.RawMessage
	dataHuman string
}

type comparisonOutputSnapshot struct {
	dataJSON  json.RawMessage
	dataHuman string
}

func (output *comparisonOutput) setData(result comparisonResult) error {
	encoded, err := json.Marshal(result)
	if err != nil {
		return err
	}
	human := fmt.Sprint(result)
	if max(len(encoded), len(human)) > comparisonMaximumOutputBytes {
		return &comparisonClassifiedError{
			kind: framework.ErrOutput, message: "output exceeds configured limit",
		}
	}
	output.mu.Lock()
	defer output.mu.Unlock()
	output.dataJSON = append(output.dataJSON[:0], encoded...)
	output.dataHuman = human

	return nil
}

func (output *comparisonOutput) render(stdout io.Writer) error {
	output.mu.Lock()
	snapshot := comparisonOutputSnapshot{
		dataJSON:  append(json.RawMessage(nil), output.dataJSON...),
		dataHuman: output.dataHuman,
	}
	output.mu.Unlock()
	err := writeComparisonJSON(stdout, struct {
		Schema string          `json:"schema"`
		OK     bool            `json:"ok"`
		Data   json.RawMessage `json:"data"`
	}{
		Schema: "go-cli/v1",
		OK:     true,
		Data:   snapshot.dataJSON,
	})
	if err != nil {
		return &comparisonClassifiedError{
			kind: framework.ErrOutput, message: "render command output: " + err.Error(), cause: err,
		}
	}

	return nil
}

func writeComparisonFailure(stdout io.Writer, failure error) error {
	kind := "usage"
	message := errComparisonUsage.Error()
	switch {
	case errors.Is(failure, errComparisonInput):
		kind = "validation"
		message = errComparisonValidation.Error()
		failure = errComparisonValidation
	case errors.Is(failure, framework.ErrOutput):
		kind = "output"
		message = failure.Error()
	case errors.Is(failure, framework.ErrUsage):
		failure = errComparisonUsage
	default:
		failure = errComparisonUsage
	}
	writeErr := writeComparisonJSON(stdout, struct {
		Schema string `json:"schema"`
		OK     bool   `json:"ok"`
		Error  struct {
			Kind    string `json:"kind"`
			Message string `json:"message"`
		} `json:"error"`
	}{
		Schema: "go-cli/v1",
		OK:     false,
		Error: struct {
			Kind    string `json:"kind"`
			Message string `json:"message"`
		}{
			Kind:    kind,
			Message: message,
		},
	})
	if writeErr != nil {
		return errors.Join(failure, &comparisonClassifiedError{
			kind: framework.ErrOutput, message: "render command output: " + writeErr.Error(),
			cause: writeErr,
		})
	}

	return failure
}

func writeComparisonJSON(stdout io.Writer, value any) error {
	encoded, err := json.Marshal(value)
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	written, err := stdout.Write(encoded)
	if err != nil {
		return err
	}
	if written != len(encoded) {
		return io.ErrShortWrite
	}

	return nil
}

type shortComparisonWriter struct{}

func (shortComparisonWriter) Write(data []byte) (int, error) {
	return max(0, len(data)-1), nil
}

func TestComparisonOutputMatchesOwnedBoundsAndWriterContract(t *testing.T) {
	t.Parallel()

	for _, testCase := range equivalentComparisonCases() {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			application, err := testCase.new()
			if err != nil {
				t.Fatal(err)
			}
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			err = application.run(context.Background(), []string{
				"tool", "deploy", "--force", "oversize",
			}, &stdout, &stderr)
			if !errors.Is(err, framework.ErrOutput) || err.Error() != "output exceeds configured limit" {
				t.Fatalf("oversized result error = %v, want output limit", err)
			}
			if got := stdout.String(); got != comparisonOutputFailureJSON() {
				t.Fatalf("oversized result stdout = %q, want %q", got, comparisonOutputFailureJSON())
			}
			if got := stderr.String(); got != "" {
				t.Fatalf("oversized result stderr = %q, want empty", got)
			}

			application, err = testCase.new()
			if err != nil {
				t.Fatal(err)
			}
			err = application.run(
				context.Background(), comparisonArgv, shortComparisonWriter{}, io.Discard,
			)
			if !errors.Is(err, framework.ErrOutput) || !errors.Is(err, io.ErrShortWrite) {
				t.Fatalf("short writer error = %v, want output-class short write", err)
			}

			application, err = testCase.new()
			if err != nil {
				t.Fatal(err)
			}
			err = application.run(
				context.Background(), []string{"tool", "deploy", "target"},
				shortComparisonWriter{}, io.Discard,
			)
			if !errors.Is(err, framework.ErrValidation) ||
				!errors.Is(err, errComparisonInput) ||
				!errors.Is(err, framework.ErrOutput) ||
				!errors.Is(err, io.ErrShortWrite) {
				t.Fatalf(
					"failure short writer error = %v, want validation/input and output/short-write classes",
					err,
				)
			}
		})
	}
}

type comparisonApplication struct {
	invoke func(context.Context, []string, io.Writer, io.Writer) error
}

func TestEquivalentBenchmarkRunnersIsolateRepeatedState(t *testing.T) {
	t.Parallel()

	for _, testCase := range equivalentComparisonCases() {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			application, err := testCase.new()
			if err != nil {
				t.Fatal(err)
			}
			if err := application.run(
				context.Background(), comparisonArgv, io.Discard, io.Discard,
			); err != nil {
				t.Fatal(err)
			}
			var invalidStdout bytes.Buffer
			var invalidStderr bytes.Buffer
			if err := application.run(
				context.Background(), []string{"tool", "deploy", "target"},
				&invalidStdout, &invalidStderr,
			); !errors.Is(err, errComparisonInput) || !errors.Is(err, framework.ErrValidation) ||
				err.Error() != errComparisonValidation.Error() {
				t.Fatalf("second invocation error = %v, want classified invalid input", err)
			}
			if got := invalidStdout.String(); got != comparisonFailureJSON() {
				t.Fatalf("second invocation stdout = %q, want %q", got, comparisonFailureJSON())
			}
			if got := invalidStderr.String(); got != "" {
				t.Fatalf("second invocation stderr = %q, want empty", got)
			}
			var stdout bytes.Buffer
			if err := application.run(
				context.Background(), comparisonArgv, &stdout, io.Discard,
			); err != nil {
				t.Fatal(err)
			}
			if got := stdout.String(); got != comparisonOutputJSON() {
				t.Fatalf("third invocation stdout = %q, want %q", got, comparisonOutputJSON())
			}
		})
	}
}

func comparisonOutputJSON() string {
	return "{\"schema\":\"go-cli/v1\",\"ok\":true," +
		"\"data\":{\"target\":\"target\",\"force\":true}}\n"
}

func comparisonFailureJSON() string {
	return "{\"schema\":\"go-cli/v1\",\"ok\":false," +
		"\"error\":{\"kind\":\"validation\",\"message\":\"command validation failed: invalid input\"}}\n"
}

func comparisonUsageFailureJSON() string {
	return "{\"schema\":\"go-cli/v1\",\"ok\":false," +
		"\"error\":{\"kind\":\"usage\",\"message\":\"missing required argument target\"}}\n"
}

func comparisonOutputFailureJSON() string {
	return "{\"schema\":\"go-cli/v1\",\"ok\":false," +
		"\"error\":{\"kind\":\"output\",\"message\":\"output exceeds configured limit\"}}\n"
}

func (application *comparisonApplication) run(
	ctx context.Context,
	argv []string,
	stdout io.Writer,
	stderr io.Writer,
) error {
	return application.invoke(ctx, argv, stdout, stderr)
}

type comparisonCase struct {
	name string
	new  func() (*comparisonApplication, error)
}

func equivalentComparisonCases() []comparisonCase {
	return []comparisonCase{
		{name: "go-cli", new: newGoCLIComparison},
		{name: "cobra", new: newCobraComparison},
		{name: "urfave-cli-v3", new: newUrfaveComparison},
		{name: "kong", new: newKongComparison},
	}
}

func TestEquivalentBenchmarkRunnersShareObservableContract(t *testing.T) {
	t.Parallel()

	wantOutput := comparisonOutputJSON()
	for _, testCase := range equivalentComparisonCases() {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			application, err := testCase.new()
			if err != nil {
				t.Fatal(err)
			}
			for invocation := range 2 {
				var stdout bytes.Buffer
				var stderr bytes.Buffer
				err = application.run(
					context.Background(), comparisonArgv, &stdout, &stderr,
				)
				if err != nil {
					t.Fatalf("invocation %d: %v", invocation+1, err)
				}
				if got := stdout.String(); got != wantOutput {
					t.Fatalf("invocation %d stdout = %q, want %q", invocation+1, got, wantOutput)
				}
				if got := stderr.String(); got != "" {
					t.Fatalf("invocation %d stderr = %q, want empty", invocation+1, got)
				}
			}
		})
	}
}

func TestEquivalentBenchmarkRunnersShareStructuralFailure(t *testing.T) {
	t.Parallel()

	for _, testCase := range equivalentComparisonCases() {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			application, err := testCase.new()
			if err != nil {
				t.Fatal(err)
			}
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			err = application.run(
				context.Background(), []string{"tool", "deploy", "--force"}, &stdout, &stderr,
			)
			if !errors.Is(err, framework.ErrUsage) || err.Error() != errComparisonUsage.Error() {
				t.Fatalf("structural error = %v, want usage-class missing target", err)
			}
			if got := stdout.String(); got != comparisonUsageFailureJSON() {
				t.Fatalf("structural stdout = %q, want %q", got, comparisonUsageFailureJSON())
			}
			if got := stderr.String(); got != "" {
				t.Fatalf("structural stderr = %q, want empty", got)
			}
		})
	}
}

func BenchmarkEquivalentColdInvocation(b *testing.B) {
	for _, benchmarkCase := range equivalentComparisonCases() {
		b.Run(benchmarkCase.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				application, err := benchmarkCase.new()
				if err != nil {
					b.Fatal(err)
				}
				err = application.run(
					context.Background(), comparisonArgv, io.Discard, io.Discard,
				)
				if err != nil {
					b.Fatal(err)
				}
			}
			reportThroughput(b)
		})
	}
}

func BenchmarkEquivalentRepeatedInvocation(b *testing.B) {
	for _, benchmarkCase := range equivalentComparisonCases() {
		b.Run(benchmarkCase.name, func(b *testing.B) {
			b.StopTimer()
			application, err := benchmarkCase.new()
			if err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.StartTimer()
			for b.Loop() {
				err = application.run(
					context.Background(), comparisonArgv, io.Discard, io.Discard,
				)
				if err != nil {
					b.Fatal(err)
				}
			}
			reportThroughput(b)
		})
	}
}

func reportThroughput(b *testing.B) {
	if elapsed := b.Elapsed().Seconds(); elapsed > 0 {
		b.ReportMetric(float64(b.N)/elapsed, "invocations/s")
	}
}

func newGoCLIComparison() (*comparisonApplication, error) {
	force := framework.BoolOption("force")
	target := framework.StringArgument("target")
	application, err := framework.Compile(framework.NewCommand(
		"tool",
		framework.WithSubcommands(framework.NewCommand(
			"deploy",
			framework.WithOptions(force),
			framework.WithArguments(target),
			framework.WithValidation(func(_ context.Context, input framework.Input) error {
				return validateComparison(force.Get(input), target.Get(input), 1)
			}),
			framework.WithHandler(func(_ context.Context, invocation framework.Invocation) error {
				return invocation.Output().SetData(comparisonResultForTarget(
					target.Get(invocation.Input()), force.Get(invocation.Input()),
				))
			}),
		)),
	))
	if err != nil {
		return nil, err
	}

	return &comparisonApplication{
		invoke: func(ctx context.Context, argv []string, stdout, stderr io.Writer) error {
			return application.RunCommand(ctx, framework.Request{
				Args: argv[1:], Stdout: stdout, Stderr: stderr,
				Output: framework.OutputPolicy{Mode: framework.OutputJSON},
			}).Err
		},
	}, nil
}

func newCobraComparison() (*comparisonApplication, error) {
	var output *comparisonOutput
	root := &cobra.Command{Use: "tool", SilenceErrors: true, SilenceUsage: true}
	var force *bool
	deploy := &cobra.Command{
		Use:  "deploy <target>",
		Args: cobra.ExactArgs(1),
		PreRunE: func(_ *cobra.Command, args []string) error {
			return validateComparison(*force, args[0], len(args))
		},
		RunE: func(_ *cobra.Command, args []string) error {
			return output.setData(comparisonResultForTarget(args[0], *force))
		},
	}
	force = deploy.Flags().Bool("force", false, "")
	root.AddCommand(deploy)
	root.CompletionOptions.DisableDefaultCmd = true

	return &comparisonApplication{
		invoke: func(ctx context.Context, argv []string, stdout, stderr io.Writer) error {
			output = new(comparisonOutput)
			*force = false
			root.SetOut(stdout)
			root.SetErr(stderr)
			root.SetArgs(argv[1:])
			if err := root.ExecuteContext(ctx); err != nil {
				return writeComparisonFailure(stdout, err)
			}

			return output.render(stdout)
		},
	}, nil
}

func newUrfaveComparison() (*comparisonApplication, error) {
	var output *comparisonOutput
	deploy := &urfave.Command{
		Name: "deploy", ArgsUsage: "<target>",
		Flags: []urfave.Flag{&urfave.BoolFlag{Name: "force"}},
		Before: func(ctx context.Context, command *urfave.Command) (context.Context, error) {
			return ctx, validateComparison(
				command.Bool("force"), command.Args().First(), command.Args().Len(),
			)
		},
		Action: func(_ context.Context, command *urfave.Command) error {
			return output.setData(comparisonResultForTarget(
				command.Args().First(), command.Bool("force"),
			))
		},
	}
	command := &urfave.Command{
		Name: "tool", HideHelp: true, HideHelpCommand: true, HideVersion: true,
		Writer: io.Discard, ErrWriter: io.Discard,
		Commands: []*urfave.Command{deploy},
	}

	return &comparisonApplication{
		invoke: func(ctx context.Context, argv []string, stdout, stderr io.Writer) error {
			output = new(comparisonOutput)
			command.Writer = stdout
			command.ErrWriter = stderr
			deploy.Writer = stdout
			deploy.ErrWriter = stderr
			if err := command.Run(ctx, argv); err != nil {
				return writeComparisonFailure(stdout, err)
			}

			return output.render(stdout)
		},
	}, nil
}

type kongCLI struct {
	Deploy kongDeploy `cmd:""`
}

type kongDeploy struct {
	Force  bool              `help:"force deployment"`
	Target string            `arg:""`
	output *comparisonOutput `kong:"-"`
}

func (command *kongDeploy) Validate() error {
	arguments := 1
	if command.Target == "" {
		arguments = 0
	}

	return validateComparison(command.Force, command.Target, arguments)
}

func (command *kongDeploy) Run() error {
	return command.output.setData(comparisonResultForTarget(command.Target, command.Force))
}

func newKongComparison() (*comparisonApplication, error) {
	model := new(kongCLI)
	parser, err := kong.New(
		model,
		kong.Name("tool"),
		kong.Writers(io.Discard, io.Discard),
		kong.Exit(func(int) {}),
		kong.NoDefaultHelp(),
	)
	if err != nil {
		return nil, err
	}

	return &comparisonApplication{
		invoke: func(_ context.Context, argv []string, stdout, _ io.Writer) error {
			model.Deploy.output = new(comparisonOutput)
			parsed, err := parser.Parse(argv[1:])
			if err != nil {
				return writeComparisonFailure(stdout, err)
			}
			if err := parsed.Run(); err != nil {
				return writeComparisonFailure(stdout, err)
			}

			return model.Deploy.output.render(stdout)
		},
	}, nil
}

func BenchmarkParsingFloorFlag(b *testing.B) {
	flags, force := newFlagParser()
	b.ReportAllocs()
	for b.Loop() {
		*force = false
		if comparisonArgv[1] != "deploy" {
			b.Fatal("unexpected command")
		}
		if err := flags.Parse(comparisonArgv[2:]); err != nil {
			b.Fatal(err)
		}
		if !*force || flags.NArg() != 1 || flags.Arg(0) != "target" {
			b.Fatal("invalid input")
		}
	}
	reportThroughput(b)
}

func newFlagParser() (*flag.FlagSet, *bool) {
	flags := flag.NewFlagSet("deploy", flag.ContinueOnError)
	flags.SetOutput(io.Discard)

	return flags, flags.Bool("force", false, "")
}
