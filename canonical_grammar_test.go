package cli_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	cli "github.com/faustbrian/go-cli/v2"
)

type invocationKey struct{}

// TestCanonicalParsingOrdersHelpAfterPrecedingFailures pins left-to-right
// parsing: help wins over later tokens and skips typed conversion, while an
// earlier grammar failure wins over a later help request.
func TestCanonicalParsingOrdersHelpAfterPrecedingFailures(t *testing.T) {
	t.Parallel()

	const sentinel = "private-sentinel"
	for _, test := range []struct {
		name     string
		args     []string
		wantErr  error
		wantCode int
		handled  bool
	}{
		{name: "help skips malformed earlier value", args: []string{"--count=bad", "--help"}, wantErr: cli.ErrHelp},
		{name: "help before unknown option", args: []string{"--help", "--unknown"}, wantErr: cli.ErrHelp},
		{name: "unknown option before help", args: []string{"--unknown", "--help"}, wantErr: cli.ErrUnknownOption, wantCode: 2},
		{name: "combined boolean shorthands", args: []string{"-qv"}, handled: true},
		{name: "single-dash long name is a shorthand cluster", args: []string{"-listen=x"}, wantErr: cli.ErrUnknownOption, wantCode: 2},
		{name: "malformed secret value", args: []string{"--token=" + sentinel}, wantErr: cli.ErrMalformedValue, wantCode: 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			handled := false
			quiet := cli.BoolOption("quiet").Short('q')
			verbose := cli.BoolOption("verbose").Short('v')
			application, err := cli.Compile(cli.NewCommand("tool",
				cli.WithOptions(cli.IntOption("count"), quiet, verbose, cli.StringOption("listen"), cli.EnumOption("token", "a", "b").Secret()),
				cli.WithHandler(func(_ context.Context, invocation cli.Invocation) error {
					handled = quiet.Get(invocation.Input()) && verbose.Get(invocation.Input())
					return nil
				}),
			))
			if err != nil {
				t.Fatal(err)
			}
			var stdout, stderr bytes.Buffer
			result := application.RunCommand(t.Context(), cli.Request{Args: test.args, Stdout: &stdout, Stderr: &stderr})
			if test.wantErr == nil && result.Err != nil || test.wantErr != nil && !errors.Is(result.Err, test.wantErr) {
				t.Fatalf("err = %v, want %v", result.Err, test.wantErr)
			}
			if result.ExitCode != test.wantCode || handled != test.handled {
				t.Fatalf("exit = %d, handled = %t", result.ExitCode, handled)
			}
			for _, rendered := range []string{stdout.String(), stderr.String(), fmt.Sprintf("%+v", result.Err)} {
				if strings.Contains(rendered, sentinel) {
					t.Fatalf("secret value rendered: %q", rendered)
				}
			}
		})
	}
}

// TestCommandSetUsesEngineGrammarWithoutOptions pins the single parser for
// command sets whose commands declare no options.
func TestCommandSetUsesEngineGrammarWithoutOptions(t *testing.T) {
	t.Parallel()

	handled := ""
	application, err := cli.CompileCommandSet(cli.CommandSet{Name: "tool", Version: "1.2.3", Commands: []cli.CommandSpec{{
		Name: "deploy", Handler: func(context.Context, cli.Invocation) error { handled = "deploy"; return nil },
	}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		args       []string
		wantErr    error
		wantCode   int
		handled    string
		unselected bool
	}{
		{args: []string{"deploy", "--"}, handled: "deploy"},
		{args: []string{"--"}},
		{args: []string{"--unknown"}, wantErr: cli.ErrUnknownOption, wantCode: 2, unselected: true},
		{args: []string{"deploy", "--unknown"}, wantErr: cli.ErrUnknownOption, wantCode: 2, unselected: true},
		{args: []string{"-"}, wantErr: cli.ErrUnknownCommand, wantCode: 2},
		{args: []string{"-5"}, wantErr: cli.ErrUnknownCommand, wantCode: 2},
		{args: []string{"--", "deploy"}, wantErr: cli.ErrUnknownCommand, wantCode: 2},
		{args: []string{"deploy", "-5"}, wantErr: cli.ErrUsage, wantCode: 2},
		{args: []string{"deploy", "-"}, wantErr: cli.ErrUsage, wantCode: 2},
		{args: []string{"deploy", "-", "--help"}, wantErr: cli.ErrHelp},
		{args: []string{"deploy", "-5", "--help"}, wantErr: cli.ErrHelp},
		{args: []string{"deploy", "--", "x"}, wantErr: cli.ErrUsage, wantCode: 2},
		{args: []string{"deploy", "extra"}, wantErr: cli.ErrUsage, wantCode: 2},
		{args: []string{"deploy", "extra", "--help"}, wantErr: cli.ErrHelp},
		{args: []string{"deploy", "extra", "--unknown"}, wantErr: cli.ErrUnknownOption, wantCode: 2, unselected: true},
		{args: []string{"--version"}, wantErr: cli.ErrVersion},
		{args: []string{"version"}, wantErr: cli.ErrVersion},
		{args: []string{"deploy", "--help"}, wantErr: cli.ErrHelp},
	} {
		handled = ""
		result := application.RunCommand(t.Context(), cli.Request{Args: test.args})
		if test.wantErr == nil && result.Err != nil || test.wantErr != nil && !errors.Is(result.Err, test.wantErr) ||
			result.ExitCode != test.wantCode || handled != test.handled ||
			test.unselected && result.Command.Name() != "" {
			t.Fatalf("%q: err %v exit %d handled %q command %q", test.args, result.Err, result.ExitCode, handled, result.Command.Name())
		}
	}

	for _, version := range []string{"1.2.3", ""} {
		rootOnly, err := cli.CompileCommandSet(cli.CommandSet{Name: "tool", Version: version})
		if err != nil {
			t.Fatal(err)
		}
		for _, test := range []struct {
			args     []string
			wantErr  error
			wantCode int
		}{
			{args: []string{"--"}},
			{args: []string{"version"}, wantErr: cli.ErrUsage, wantCode: 2},
			{args: []string{"foo"}, wantErr: cli.ErrUsage, wantCode: 2},
			{args: []string{"-"}, wantErr: cli.ErrUsage, wantCode: 2},
			{args: []string{"-5"}, wantErr: cli.ErrUsage, wantCode: 2},
			{args: []string{"--", "x"}, wantErr: cli.ErrUsage, wantCode: 2},
			{args: []string{"foo", "--help"}, wantErr: cli.ErrHelp},
			{args: []string{"version", "--help"}, wantErr: cli.ErrHelp},
		} {
			result := rootOnly.RunCommand(t.Context(), cli.Request{Args: test.args})
			if test.wantErr == nil && result.Err != nil || test.wantErr != nil && !errors.Is(result.Err, test.wantErr) ||
				result.ExitCode != test.wantCode {
				t.Fatalf("root-only (version %q) %q: err %v exit %d", version, test.args, result.Err, result.ExitCode)
			}
		}
	}
	rootOnly, err := cli.CompileCommandSet(cli.CommandSet{Name: "tool", Version: "1.2.3"})
	if err != nil {
		t.Fatal(err)
	}
	if result := rootOnly.RunCommand(t.Context(), cli.Request{Args: []string{"--version"}}); !errors.Is(result.Err, cli.ErrVersion) || result.ExitCode != 0 {
		t.Fatalf("root-only --version = %v exit %d", result.Err, result.ExitCode)
	}
}

// TestCanonicalCommandReplacesFlagCommandGrammar pins how former
// CompileFlagCommand invocations behave under the canonical grammar.
func TestCanonicalCommandReplacesFlagCommandGrammar(t *testing.T) {
	t.Parallel()

	count := cli.IntOption("count")
	h := cli.StringOption("h")
	var ran bool
	var gotCount int64
	var gotH string
	application, err := cli.Compile(cli.NewCommand("service", cli.WithOptions(count, h), cli.WithHandler(func(_ context.Context, invocation cli.Invocation) error {
		ran = true
		gotCount = count.Get(invocation.Input())
		gotH = h.Get(invocation.Input())
		return nil
	})))
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		args    []string
		wantErr error
		ran     bool
		count   int64
		h       string
		code    int
	}{
		{args: []string{"--count=bad", "--count=1"}, ran: true, count: 1},
		{args: []string{"extra", "--help"}, wantErr: cli.ErrHelp},
		{args: []string{"-h", "opaque"}, wantErr: cli.ErrHelp},
		{args: []string{"--h", "opaque"}, ran: true, h: "opaque"},
		{args: []string{"-help"}, wantErr: cli.ErrUnknownOption, code: 2},
		{args: []string{"-help=false"}, wantErr: cli.ErrUnknownOption, code: 2},
		{args: []string{"--help=false"}, wantErr: cli.ErrUnknownOption, code: 2},
		{args: []string{"-h=false"}, wantErr: cli.ErrUnknownOption, code: 2},
	} {
		ran, gotCount, gotH = false, 0, ""
		result := application.Run(t.Context(), cli.Request{Args: test.args})
		if test.wantErr == nil && result.Err != nil || test.wantErr != nil && !errors.Is(result.Err, test.wantErr) ||
			result.ExitCode != test.code || ran != test.ran || gotCount != test.count || gotH != test.h {
			t.Fatalf("%q: err %v exit %d ran %t count %d h %q", test.args, result.Err, result.ExitCode, ran, gotCount, gotH)
		}
	}
}

// TestCanonicalConcurrentInputsRemainInvocationLocal runs concurrent
// invocations through every runtime surface; each handler must see its own value.
func TestCanonicalConcurrentInputsRemainInvocationLocal(t *testing.T) {
	t.Parallel()

	value := cli.StringOption("value")
	check := func(ctx context.Context, invocation cli.Invocation) error {
		if value.Get(invocation.Input()) != ctx.Value(invocationKey{}).(string) {
			t.Error("option values crossed invocations")
		}
		return nil
	}
	application, err := cli.Compile(cli.NewCommand("service", cli.WithOptions(value), cli.WithHandler(check)))
	if err != nil {
		t.Fatal(err)
	}
	set, err := cli.CompileCommandSet(cli.CommandSet{Name: "service", Commands: []cli.CommandSpec{{
		Name: "serve", Options: []cli.OptionDefinition{value}, Handler: check,
	}}})
	if err != nil {
		t.Fatal(err)
	}
	var joined sync.WaitGroup
	for index := range 16 {
		joined.Go(func() {
			text := fmt.Sprint(index)
			ctx := context.WithValue(t.Context(), invocationKey{}, text)
			for _, result := range []cli.Result{
				application.RunCommand(ctx, cli.Request{Args: []string{"--value=" + text}}),
				application.Run(ctx, cli.Request{Args: []string{"--value=" + text}}),
				set.RunCommand(ctx, cli.Request{Args: []string{"serve", "--value=" + text}}),
			} {
				if result.Err != nil {
					t.Error(result.Err)
				}
			}
		})
	}
	joined.Wait()
}
