package cli_test

import (
	"context"
	"errors"
	"io"
	"strconv"
	"testing"

	cli "github.com/faustbrian/go-cli"
)

// BenchmarkCompileSmallTree builds and compiles four children with three ordinary options
// per child inside b.Loop, so construction is part of the measured operation.
func BenchmarkCompileSmallTree(b *testing.B) {
	for b.Loop() {
		_, err := cli.Compile(benchmarkTree(4, 3))
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkCompileLargeTree includes construction and compilation of 64 direct children
// with 12 ordinary options each; it is not reused-invocation latency.
func BenchmarkCompileLargeTree(b *testing.B) {
	for b.Loop() {
		_, err := cli.Compile(benchmarkTree(64, 12))
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkCompileShapes includes construction/compilation of explicit broad, deep
// and default-command-count boundary trees, not equivalent-shape rankings.
func BenchmarkCompileShapes(b *testing.B) {
	b.Run("broad", func(b *testing.B) {
		for b.Loop() {
			if _, err := cli.Compile(benchmarkTree(1024, 1)); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("deep", func(b *testing.B) {
		for b.Loop() {
			if _, err := cli.Compile(benchmarkDeepTree(63)); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("maximum-commands", func(b *testing.B) {
		for b.Loop() {
			if _, err := cli.Compile(benchmarkTree(4095, 0)); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkDispatch uses a precompiled 16-child/eight-option tree and a final-child
// integer/positional argv with discarded IO; b.Loop excludes construction.
func BenchmarkDispatch(b *testing.B) {
	application, err := cli.Compile(benchmarkTree(16, 8))
	if err != nil {
		b.Fatal(err)
	}
	request := cli.Request{
		Args:   []string{"command-15", "--count", "42", "target"},
		Stdout: io.Discard, Stderr: io.Discard,
	}
	b.ReportAllocs()
	for b.Loop() {
		if result := application.Run(context.Background(), request); result.Err != nil {
			b.Fatal(result.Err)
		}
	}
}

// BenchmarkDeepDispatch uses a precompiled 17-token path with discarded IO; b.Loop
// excludes graph compilation and measures invocation work only.
func BenchmarkDeepDispatch(b *testing.B) {
	leaf := cli.NewCommand("leaf")
	argv := []string{"leaf"}
	for depth := 0; depth < 16; depth++ {
		name := "level-" + strconv.Itoa(depth)
		leaf = cli.NewCommand(name, cli.WithSubcommands(leaf))
		argv = append([]string{name}, argv...)
	}
	root := cli.NewCommand("tool", cli.WithSubcommands(leaf))
	application, err := cli.Compile(root)
	if err != nil {
		b.Fatal(err)
	}
	request := cli.Request{Args: argv, Stdout: io.Discard, Stderr: io.Discard}
	b.ReportAllocs()
	for b.Loop() {
		if result := application.Run(context.Background(), request); result.Err != nil {
			b.Fatal(result.Err)
		}
	}
}

// BenchmarkGeneration reuses compiled 64-child metadata and measures help width 80,
// manifest or static partial completion, excluding construction via b.Loop.
func BenchmarkGeneration(b *testing.B) {
	application, err := cli.Compile(benchmarkTree(64, 12))
	if err != nil {
		b.Fatal(err)
	}
	b.Run("help", func(b *testing.B) {
		for b.Loop() {
			if _, err := application.Help(nil, cli.HelpOptions{Width: 80}); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("manifest", func(b *testing.B) {
		for b.Loop() {
			if _, err := application.ManifestJSON(); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("completion", func(b *testing.B) {
		for b.Loop() {
			if _, err := application.Complete(context.Background(), []string{"command-"}); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkExecutionScenarios reuses precompiled fixed phase/error/JSON/suggestion/
// cancellation cases with discarded IO; setup is excluded by b.Loop.
func BenchmarkExecutionScenarios(b *testing.B) {
	conversion, err := cli.Compile(cli.NewCommand(
		"tool",
		cli.WithOptions(cli.IntOption("count")),
		cli.WithHandler(func(context.Context, cli.Invocation) error { return nil }),
	))
	if err != nil {
		b.Fatal(err)
	}
	validationFailure := errors.New("invalid request")
	validation, err := cli.Compile(cli.NewCommand(
		"tool",
		cli.WithValidation(func(context.Context, cli.Input) error { return validationFailure }),
	))
	if err != nil {
		b.Fatal(err)
	}
	jsonOutput, err := cli.Compile(cli.NewCommand(
		"tool",
		cli.WithHandler(func(_ context.Context, invocation cli.Invocation) error {
			return invocation.Output().SetData(struct {
				Value string `json:"value"`
			}{Value: "result"})
		}),
	))
	if err != nil {
		b.Fatal(err)
	}
	suggestions, err := cli.Compile(cli.NewCommand(
		"tool",
		cli.WithSubcommands(
			cli.NewCommand("deploy"),
			cli.NewCommand("describe"),
			cli.NewCommand("destroy"),
		),
	))
	if err != nil {
		b.Fatal(err)
	}
	canceledContext, cancel := context.WithCancel(context.Background())
	cancel()
	cases := []struct {
		name        string
		application *cli.Application
		ctx         context.Context
		request     cli.Request
		wantError   bool
	}{
		{"typed-conversion", conversion, context.Background(), benchmarkRequest("--count", "42"), false},
		{"usage-error", conversion, context.Background(), benchmarkRequest("--unknown"), true},
		{"validation-error", validation, context.Background(), benchmarkRequest(), true},
		{"suggestion", suggestions, context.Background(), benchmarkRequest("deply"), true},
		{"json-output", jsonOutput, context.Background(), cli.Request{
			Stdout: io.Discard, Stderr: io.Discard,
			Output: cli.OutputPolicy{Mode: cli.OutputJSON},
		}, false},
		{"cancellation", conversion, canceledContext, benchmarkRequest(), true},
	}
	for _, test := range cases {
		b.Run(test.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				result := test.application.Run(test.ctx, test.request)
				if (result.Err != nil) != test.wantError {
					b.Fatalf("Run() error = %v, want error %t", result.Err, test.wantError)
				}
			}
		})
	}
}

func benchmarkRequest(args ...string) cli.Request {
	return cli.Request{Args: args, Stdout: io.Discard, Stderr: io.Discard}
}

func benchmarkDeepTree(depth int) *cli.Command {
	command := cli.NewCommand("leaf")
	for index := 0; index < depth; index++ {
		command = cli.NewCommand(
			"level-"+strconv.Itoa(index),
			cli.WithSubcommands(command),
		)
	}
	return command
}

func benchmarkTree(commands, options int) *cli.Command {
	children := make([]*cli.Command, 0, commands)
	for commandIndex := 0; commandIndex < commands; commandIndex++ {
		definitions := make([]cli.OptionDefinition, 0, options)
		for optionIndex := 0; optionIndex < options; optionIndex++ {
			definitions = append(definitions, cli.StringOption(
				"option-"+strconv.Itoa(optionIndex),
			))
		}
		if commandIndex == commands-1 {
			definitions = append(definitions, cli.IntOption("count"))
		}
		children = append(children, cli.NewCommand(
			"command-"+strconv.Itoa(commandIndex),
			cli.WithOptions(definitions...),
			cli.WithArguments(cli.StringArgument("target")),
		))
	}

	return cli.NewCommand("tool", cli.WithSubcommands(children...))
}
