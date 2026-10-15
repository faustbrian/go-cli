package cli_test

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"

	cli "github.com/faustbrian/go-cli"
)

// TestFlagCommandPreservesServiceFlags uses the native flag package as the
// independent oracle, including failure precedence, not just accepted spelling.
func TestFlagCommandPreservesServiceFlags(t *testing.T) {
	t.Parallel()
	for _, argv := range [][]string{
		{}, {"-listen=127.0.0.1:9000"},
		{"-listen", "127.0.0.1:9001", "-diagnostic"},
		{"-diagnostic=false", "--listen=127.0.0.1:9002"},
		{"--diagnostic=true", "-diagnostic=0"},
		{"-listen=first", "--listen=last", "-diagnostic=1"},
		{"-listen="}, {"-listen", "--help"},
		{"-help"}, {"-help=false"}, {"--help=false"}, {"-h=false"},
		{"-diagnostic=invalid"}, {"-listen"}, {"-unknown=private-sentinel"},
		{"--", "-diagnostic"}, {"unexpected", "--help"},
		{"--diagnostic=invalid", "--help"},
		{"--diagnostic=invalid", "--diagnostic=true"},
		{"__complete"}, {"__completeNoDesc"}, {"--version"},
	} {
		t.Run(fmt.Sprint(argv), func(t *testing.T) {
			standard := flag.NewFlagSet("service", flag.ContinueOnError)
			standard.SetOutput(io.Discard)
			wantListen := standard.String("listen", "127.0.0.1:8080", "listen address")
			wantDiagnostic := standard.Bool("diagnostic", false, "diagnostic mode")
			standardErr := standard.Parse(argv)
			wantHelp := errors.Is(standardErr, flag.ErrHelp)
			wantRun := standardErr == nil && standard.NArg() == 0

			listen := cli.StringOption("listen").Default("127.0.0.1:8080")
			diagnostic := cli.BoolOption("diagnostic")
			called := false
			application, err := cli.CompileFlagCommand(cli.NewCommand("service",
				cli.WithOptions(listen, diagnostic),
				cli.WithVersion("1.2.3"),
				cli.WithHandler(func(_ context.Context, invocation cli.Invocation) error {
					called = true
					if listen.Get(invocation.Input()) != *wantListen || diagnostic.Get(invocation.Input()) != *wantDiagnostic {
						t.Fatal("service flag values differ from the standard flag contract")
					}
					return nil
				}),
			))
			if err != nil {
				t.Fatal(err)
			}
			for _, runner := range []struct {
				name string
				run  func(context.Context, cli.Request) cli.Result
			}{
				{"RunCommand", application.RunCommand},
				{"Run", application.Run},
			} {
				t.Run(runner.name, func(t *testing.T) {
					called = false
					var output bytes.Buffer
					result := runner.run(t.Context(), cli.Request{Args: argv, Stdout: &output, Stderr: &output})
					if called != wantRun || (result.ExitCode == 0) != (wantRun || wantHelp) || errors.Is(result.Err, cli.ErrHelp) != wantHelp {
						t.Fatalf("%s() = (called=%t, exit=%d, err=%v), want (run=%t, help=%t)", runner.name, called, result.ExitCode, result.Err, wantRun, wantHelp)
					}
				})
			}
		})
	}
}

func TestFlagCommandRejectsUnsupportedGrammar(t *testing.T) {
	t.Parallel()
	for _, root := range []*cli.Command{
		nil,
		cli.NewCommand("service", cli.WithSubcommands(cli.NewCommand("serve"))),
		cli.NewCommand("service", cli.WithArguments(cli.StringArgument("target"))),
		cli.NewCommand("service", cli.WithOptions(cli.BoolOption("verbose").Short('v'))),
	} {
		if application, err := cli.CompileFlagCommand(root); application != nil || !errors.Is(err, cli.ErrInternal) {
			t.Fatal("unsupported flag grammar did not fail construction")
		}
	}
	quiet := cli.BoolOption("quiet").Short('q')
	verbose := cli.BoolOption("verbose").Short('v')
	called := false
	normal, err := cli.Compile(cli.NewCommand("service", cli.WithOptions(quiet, verbose), cli.WithHandler(func(_ context.Context, invocation cli.Invocation) error {
		called = true
		if !quiet.Get(invocation.Input()) || !verbose.Get(invocation.Input()) {
			t.Fatal("normal shorthand cluster grammar changed")
		}
		return nil
	})))
	if err != nil {
		t.Fatal(err)
	}
	if result := normal.RunCommand(t.Context(), cli.Request{Args: []string{"-qv"}}); result.Err != nil || !called {
		t.Fatal("normal command path changed")
	}
}

func TestFlagCommandConvertsEachOccurrenceOnceAndBoundsBeforeParsing(t *testing.T) {
	t.Parallel()
	parses, calls := 0, 0
	value := cli.TypedOption("value", "string", func(raw string) (string, error) { parses++; return raw, nil })
	application, err := cli.CompileFlagCommand(cli.NewCommand("service", cli.WithOptions(value), cli.WithHandler(func(_ context.Context, invocation cli.Invocation) error {
		calls++
		if value.Get(invocation.Input()) != "last" {
			t.Fatal("last scalar occurrence lost")
		}
		return nil
	})), cli.WithLimits(cli.Limits{MaximumArgvBytes: 64}))
	if err != nil {
		t.Fatal(err)
	}
	if result := application.RunCommand(t.Context(), cli.Request{Args: []string{"-value=first", "-value=last"}}); result.Err != nil || calls != 1 || parses != 2 {
		t.Fatal("typed parser was repeated during input resolution")
	}
	if result := application.RunCommand(t.Context(), cli.Request{Args: []string{"-value=first", "--help"}}); !errors.Is(result.Err, cli.ErrHelp) || calls != 1 || parses != 3 {
		t.Fatal("help changed preceding conversion ownership")
	}
	if result := application.RunCommand(t.Context(), cli.Request{Args: []string{"-value=" + strings.Repeat("x", 100)}}); !errors.Is(result.Err, cli.ErrUsage) || parses != 3 || calls != 1 {
		t.Fatal("oversized argv reached native parsing")
	}
}

func TestFlagCommandPreservesPrivateErrorsAndRegisteredHelpName(t *testing.T) {
	t.Parallel()
	token := cli.EnumOption("token", "accepted").Secret()
	header := cli.StringOption("h")
	calls := 0
	application, err := cli.CompileFlagCommand(cli.NewCommand("service", cli.WithOptions(token, header), cli.WithHandler(func(_ context.Context, invocation cli.Invocation) error {
		calls++
		if header.Get(invocation.Input()) != "opaque" {
			t.Fatal("registered h name was mistaken for help")
		}
		return nil
	})))
	if err != nil {
		t.Fatal(err)
	}
	for _, argv := range [][]string{{"--h=opaque"}, {"-h", "opaque"}} {
		if result := application.RunCommand(t.Context(), cli.Request{Args: argv}); result.Err != nil {
			t.Fatal(result.Err)
		}
	}
	for _, scenario := range []struct {
		argv []string
		want error
	}{
		{[]string{"-token=private-sentinel", "--help"}, cli.ErrMalformedValue},
		{[]string{"-unknown=private-sentinel"}, cli.ErrUsage},
		{[]string{"-token"}, cli.ErrUsage},
	} {
		var output bytes.Buffer
		result := application.RunCommand(t.Context(), cli.Request{Args: scenario.argv, Stdout: &output, Stderr: &output})
		if calls != 2 || !errors.Is(result.Err, scenario.want) || strings.Contains(output.String()+fmt.Sprintf("%+v", result.Err), "private-sentinel") {
			t.Fatal("native flag failure leaked input or executed work")
		}
	}
}

type flagInvocationKey struct{}

func TestFlagCommandConcurrentInputsRemainInvocationLocal(t *testing.T) {
	t.Parallel()
	value := cli.StringOption("value")
	application, err := cli.CompileFlagCommand(cli.NewCommand("service", cli.WithOptions(value), cli.WithHandler(func(ctx context.Context, invocation cli.Invocation) error {
		if value.Get(invocation.Input()) != ctx.Value(flagInvocationKey{}).(string) {
			t.Error("native flag values crossed invocations")
		}
		return nil
	})))
	if err != nil {
		t.Fatal(err)
	}
	var joined sync.WaitGroup
	for index := range 16 {
		joined.Go(func() {
			text := fmt.Sprint(index)
			ctx := context.WithValue(t.Context(), flagInvocationKey{}, text)
			if result := application.RunCommand(ctx, cli.Request{Args: []string{"-value=" + text}}); result.Err != nil {
				t.Error(result.Err)
			}
		})
	}
	joined.Wait()
}
