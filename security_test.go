package cli_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	cli "github.com/faustbrian/go-cli"
)

func TestSecretCallbackFailuresOmitSensitiveValues(t *testing.T) {
	t.Parallel()

	const secretValue = "vault-token-should-not-escape"
	tests := []struct {
		name    string
		options func(error) []cli.CommandOption
	}{
		{
			name: "validation",
			options: func(failure error) []cli.CommandOption {
				return []cli.CommandOption{
					cli.WithValidation(func(context.Context, cli.Input) error { return failure }),
				}
			},
		},
		{
			name: "pre-run hook",
			options: func(failure error) []cli.CommandOption {
				return []cli.CommandOption{
					cli.WithPreRun(func(context.Context, cli.Invocation) error { return failure }),
				}
			},
		},
		{
			name: "middleware",
			options: func(failure error) []cli.CommandOption {
				return []cli.CommandOption{
					cli.WithMiddleware(func(context.Context, cli.CommandMetadata, cli.Next) error {
						return failure
					}),
				}
			},
		},
		{
			name: "handler",
			options: func(failure error) []cli.CommandOption {
				return []cli.CommandOption{
					cli.WithHandler(func(context.Context, cli.Invocation) error { return failure }),
				}
			},
		},
		{
			name: "post-run hook",
			options: func(failure error) []cli.CommandOption {
				return []cli.CommandOption{
					cli.WithPostRun(func(context.Context, cli.Invocation) error { return failure }),
				}
			},
		},
		{
			name: "cleanup hook",
			options: func(failure error) []cli.CommandOption {
				return []cli.CommandOption{
					cli.WithCleanup(func(context.Context, cli.Invocation) error { return failure }),
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			failure := &secretCallbackFailure{value: secretValue}
			secret := cli.StringOption("token").Secret()
			options := []cli.CommandOption{
				cli.WithOptions(secret),
				cli.WithHandler(func(context.Context, cli.Invocation) error { return nil }),
			}
			options = append(options, test.options(failure)...)
			application, err := cli.Compile(cli.NewCommand("tool", options...))
			if err != nil {
				t.Fatalf("Compile() error = %v", err)
			}

			stderr := new(bytes.Buffer)
			result := application.Run(context.Background(), cli.Request{
				Args:   []string{"--token", secretValue},
				Stderr: stderr,
			})
			if result.Err == nil || !errors.Is(result.Err, failure) {
				t.Fatalf("Run() error = %v, want callback identity", result.Err)
			}
			assertErrorChainOmits(t, result.Err, secretValue)
			if diagnostic := fmt.Sprintf("%#v", result.Err); strings.Contains(diagnostic, secretValue) {
				t.Fatalf("Go-syntax diagnostic contains secret value: %q", diagnostic)
			}
			var exposed *secretCallbackFailure
			if errors.As(result.Err, &exposed) {
				t.Fatal("errors.As exposed the secret-bearing callback cause")
			}
			assertProtectedUnwrap(t, result.Err, secretValue)
			if strings.Contains(stderr.String(), secretValue) {
				t.Fatalf("stderr contains secret value: %q", stderr.String())
			}
		})
	}
}

func TestSecretArgumentProtectsHandlerFailure(t *testing.T) {
	t.Parallel()

	const secretValue = "argument-secret-should-not-escape"
	failure := &secretCallbackFailure{value: secretValue}
	secret := cli.StringArgument("token").Secret()
	application, err := cli.Compile(cli.NewCommand(
		"tool",
		cli.WithArguments(secret),
		cli.WithHandler(func(context.Context, cli.Invocation) error { return failure }),
	))
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	result := application.Run(context.Background(), cli.Request{Args: []string{secretValue}})
	if !errors.Is(result.Err, failure) {
		t.Fatalf("Run() error = %v, want callback identity", result.Err)
	}
	assertErrorChainOmits(t, result.Err, secretValue)
}

func TestProtectedCauseGoSyntaxOmitsValueTypedSecrets(t *testing.T) {
	t.Parallel()

	const redactionMarker = "value-typed-cause-secret-should-not-escape"
	failure := secretValueFailure{value: redactionMarker}
	application, err := cli.Compile(cli.NewCommand(
		"tool",
		cli.WithOptions(cli.StringOption("token").Secret()),
		cli.WithHandler(func(context.Context, cli.Invocation) error { return failure }),
	))
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	result := application.Run(context.Background(), cli.Request{})
	if !errors.Is(result.Err, failure) {
		t.Fatalf("Run() error = %v, want value cause identity", result.Err)
	}
	cause := errors.Unwrap(result.Err)
	if diagnostic := fmt.Sprintf("%#v", cause); strings.Contains(diagnostic, redactionMarker) {
		t.Fatalf("protected value cause diagnostic contains secret value: %q", diagnostic)
	}
}

func TestSecretCommandProtectsOutputWriterFailure(t *testing.T) {
	t.Parallel()

	const secretValue = "writer-secret-should-not-escape"
	failure := &secretCallbackFailure{value: secretValue}
	application, err := cli.Compile(cli.NewCommand(
		"tool",
		cli.WithOptions(cli.StringOption("token").Secret()),
		cli.WithHandler(func(_ context.Context, invocation cli.Invocation) error {
			return invocation.Output().SetData("complete")
		}),
	))
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	result := application.Run(context.Background(), cli.Request{
		Args:   []string{"--token", "safe-input"},
		Stdout: failingSecretWriter{err: failure},
	})
	if !errors.Is(result.Err, failure) {
		t.Fatalf("Run() error = %v, want writer identity", result.Err)
	}
	assertErrorChainOmits(t, result.Err, secretValue)
	var exposed *secretCallbackFailure
	if errors.As(result.Err, &exposed) {
		t.Fatal("errors.As exposed the secret-bearing writer cause")
	}
	assertProtectedUnwrap(t, result.Err, secretValue)
}

func TestSecretCommandProtectsCancellationCause(t *testing.T) {
	t.Parallel()

	const secretValue = "cancellation-secret-should-not-escape"
	failure := &secretCallbackFailure{value: secretValue}
	ctx, cancel := context.WithCancelCause(context.Background())
	application, err := cli.Compile(cli.NewCommand(
		"tool",
		cli.WithOptions(cli.StringOption("token").Secret()),
		cli.WithHandler(func(context.Context, cli.Invocation) error {
			cancel(failure)

			return context.Canceled
		}),
	))
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	result := application.Run(ctx, cli.Request{Args: []string{"--token", "safe-input"}})
	if !errors.Is(result.Err, failure) || !errors.Is(result.Err, context.Canceled) {
		t.Fatalf("Run() error = %v, want cancellation and cause identity", result.Err)
	}
	assertErrorChainOmits(t, result.Err, secretValue)
	var exposed *secretCallbackFailure
	if errors.As(result.Err, &exposed) {
		t.Fatal("errors.As exposed the secret-bearing cancellation cause")
	}
	assertProtectedUnwrap(t, result.Err, secretValue)
}

func TestSecretCommandProtectsPreCanceledCause(t *testing.T) {
	t.Parallel()

	const secretValue = "pre-canceled-secret-should-not-escape"
	failure := &secretCallbackFailure{value: secretValue}
	ctx, cancel := context.WithCancelCause(context.Background())
	cancel(failure)
	application, err := cli.Compile(cli.NewCommand(
		"tool",
		cli.WithOptions(cli.StringOption("token").Secret()),
	))
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	result := application.Run(ctx, cli.Request{})
	if !errors.Is(result.Err, failure) || !errors.Is(result.Err, context.Canceled) {
		t.Fatalf("Run() error = %v, want cancellation and cause identity", result.Err)
	}
	assertErrorChainOmits(t, result.Err, secretValue)
	var exposed *secretCallbackFailure
	if errors.As(result.Err, &exposed) {
		t.Fatal("errors.As exposed the pre-canceled secret-bearing cause")
	}
	assertProtectedUnwrap(t, result.Err, secretValue)
}

func TestSecretCommandProtectsPreCanceledCompletionCause(t *testing.T) {
	t.Parallel()

	const secretValue = "pre-canceled-completion-secret-should-not-escape"
	failure := &secretCallbackFailure{value: secretValue}
	ctx, cancel := context.WithCancelCause(context.Background())
	cancel(failure)
	application, err := cli.Compile(cli.NewCommand(
		"tool",
		cli.WithOptions(cli.StringOption("token").Secret()),
	))
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	_, completionErr := application.Complete(ctx, []string{""})
	if !errors.Is(completionErr, failure) || !errors.Is(completionErr, context.Canceled) {
		t.Fatalf("Complete() error = %v, want cancellation and cause identity", completionErr)
	}
	assertErrorChainOmits(t, completionErr, secretValue)
	var exposed *secretCallbackFailure
	if errors.As(completionErr, &exposed) {
		t.Fatal("errors.As exposed the pre-canceled completion cause")
	}
	assertProtectedUnwrap(t, completionErr, secretValue)
}

func TestSecretCommandSetProtectsCancellationCause(t *testing.T) {
	t.Parallel()

	const secretValue = "command-set-cancellation-secret-should-not-escape"
	failure := &secretCallbackFailure{value: secretValue}
	ctx, cancel := context.WithCancelCause(context.Background())
	application, err := cli.CompileCommandSet(cli.CommandSet{
		Name: "tool",
		Commands: []cli.CommandSpec{{
			Name:    "deploy",
			Options: []cli.OptionDefinition{cli.StringOption("token").Secret()},
			Handler: func(context.Context, cli.Invocation) error {
				cancel(failure)

				return nil
			},
		}},
	})
	if err != nil {
		t.Fatalf("CompileCommandSet() error = %v", err)
	}

	result := application.RunCommand(ctx, cli.Request{
		Args: []string{"deploy", "--token", "safe-input"},
	})
	if !errors.Is(result.Err, failure) || !errors.Is(result.Err, context.Canceled) {
		t.Fatalf("RunCommand() error = %v, want cancellation and cause identity", result.Err)
	}
	assertErrorChainOmits(t, result.Err, secretValue)
	var exposed *secretCallbackFailure
	if errors.As(result.Err, &exposed) {
		t.Fatal("errors.As exposed the secret-bearing command-set cancellation cause")
	}
	assertProtectedUnwrap(t, result.Err, secretValue)
}

func TestSecretCommandSetProtectsPreCanceledCause(t *testing.T) {
	t.Parallel()

	const secretValue = "pre-canceled-command-set-secret-should-not-escape"
	failure := &secretCallbackFailure{value: secretValue}
	ctx, cancel := context.WithCancelCause(context.Background())
	cancel(failure)
	application, err := cli.CompileCommandSet(cli.CommandSet{
		Name: "tool",
		Commands: []cli.CommandSpec{{
			Name:    "deploy",
			Options: []cli.OptionDefinition{cli.StringOption("token").Secret()},
			Handler: func(context.Context, cli.Invocation) error { return nil },
		}},
	})
	if err != nil {
		t.Fatalf("CompileCommandSet() error = %v", err)
	}

	result := application.RunCommand(ctx, cli.Request{Args: []string{"deploy"}})
	if !errors.Is(result.Err, failure) || !errors.Is(result.Err, context.Canceled) {
		t.Fatalf("RunCommand() error = %v, want cancellation and cause identity", result.Err)
	}
	assertErrorChainOmits(t, result.Err, secretValue)
	var exposed *secretCallbackFailure
	if errors.As(result.Err, &exposed) {
		t.Fatal("errors.As exposed the pre-canceled command-set secret-bearing cause")
	}
	assertProtectedUnwrap(t, result.Err, secretValue)
}

func TestSecretApplicationsProtectPreselectionWriterFailures(t *testing.T) {
	t.Parallel()

	application, err := cli.Compile(cli.NewCommand(
		"tool",
		cli.WithSubcommands(cli.NewCommand(
			"deploy",
			cli.WithOptions(cli.StringOption("token").Secret()),
		)),
	))
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	commandSet, err := cli.CompileCommandSet(cli.CommandSet{
		Name: "tool",
		Commands: []cli.CommandSpec{{
			Name:    "deploy",
			Options: []cli.OptionDefinition{cli.StringOption("token").Secret()},
			Handler: func(context.Context, cli.Invocation) error { return nil },
		}},
	})
	if err != nil {
		t.Fatalf("CompileCommandSet() error = %v", err)
	}

	for name, run := range map[string]func(context.Context, cli.Request) cli.Result{
		"application": application.Run,
		"command set": commandSet.RunCommand,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			secretValue := name + "-preselection-writer-secret-should-not-escape"
			failure := &secretCallbackFailure{value: secretValue}
			ctx, cancel := context.WithCancelCause(context.Background())
			cancel(failure)
			result := run(ctx, cli.Request{Stderr: failingSecretWriter{err: failure}})
			if !errors.Is(result.Err, failure) || !errors.Is(result.Err, context.Canceled) {
				t.Fatalf("preselection error = %v, want cancellation and writer cause identity", result.Err)
			}
			assertErrorChainOmits(t, result.Err, secretValue)
			var exposed *secretCallbackFailure
			if errors.As(result.Err, &exposed) {
				t.Fatal("errors.As exposed the preselection writer cause")
			}
		})
	}
}

func TestSecretCommandProtectsAuxiliaryWriterFailures(t *testing.T) {
	t.Parallel()

	application, err := cli.Compile(cli.NewCommand(
		"tool",
		cli.WithOptions(cli.StringOption("token").Secret()),
	))
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	for name, args := range map[string][]string{
		"help":       {"--help"},
		"completion": {"__complete", ""},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			secretValue := name + "-writer-secret-should-not-escape"
			failure := &secretCallbackFailure{value: secretValue}
			result := application.Run(context.Background(), cli.Request{
				Args:   args,
				Stdout: failingSecretWriter{err: failure},
			})
			if !errors.Is(result.Err, failure) {
				t.Fatalf("Run() error = %v, want writer identity", result.Err)
			}
			assertErrorChainOmits(t, result.Err, secretValue)
			var exposed *secretCallbackFailure
			if errors.As(result.Err, &exposed) {
				t.Fatal("errors.As exposed the secret-bearing writer cause")
			}
			assertProtectedUnwrap(t, result.Err, secretValue)
		})
	}
}

func TestSecretChildProtectsCompletionProtocolWriterFailure(t *testing.T) {
	t.Parallel()

	application, err := cli.Compile(cli.NewCommand(
		"tool",
		cli.WithSubcommands(cli.NewCommand(
			"deploy",
			cli.WithOptions(cli.StringOption("token").Secret()),
		)),
	))
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	for name, args := range map[string][]string{
		"before child selection": {"__complete", ""},
		"selected child":         {"__complete", "deploy", ""},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			secretValue := name + "-completion-writer-secret-should-not-escape"
			failure := &secretCallbackFailure{value: secretValue}
			result := application.Run(context.Background(), cli.Request{
				Args:   args,
				Stdout: failingSecretWriter{err: failure},
			})
			if !errors.Is(result.Err, failure) {
				t.Fatalf("Run() error = %v, want writer identity", result.Err)
			}
			assertErrorChainOmits(t, result.Err, secretValue)
			var exposed *secretCallbackFailure
			if errors.As(result.Err, &exposed) {
				t.Fatal("errors.As exposed the secret-bearing child completion writer cause")
			}
			assertProtectedUnwrap(t, result.Err, secretValue)
		})
	}
}

func TestSecretCommandProtectsCompletionProviderFailure(t *testing.T) {
	t.Parallel()

	const secretValue = "completion-provider-secret-should-not-escape"
	failure := &secretCallbackFailure{value: secretValue}
	application, err := cli.Compile(cli.NewCommand(
		"tool",
		cli.WithOptions(
			cli.StringOption("token").Secret(),
			cli.StringOption("region").Completion(func(
				context.Context,
				cli.CompletionRequest,
			) ([]cli.CompletionCandidate, error) {
				return nil, failure
			}),
		),
	))
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	_, completionErr := application.Complete(
		context.Background(), []string{"--region", ""},
	)
	if !errors.Is(completionErr, failure) {
		t.Fatalf("Complete() error = %v, want provider identity", completionErr)
	}
	assertErrorChainOmits(t, completionErr, secretValue)
	var exposed *secretCallbackFailure
	if errors.As(completionErr, &exposed) {
		t.Fatal("errors.As exposed the secret-bearing completion provider cause")
	}
	assertProtectedUnwrap(t, completionErr, secretValue)
}

func TestSecretCommandProtectsCompletionCancellation(t *testing.T) {
	t.Parallel()

	for name, provider := range map[string]func(
		context.CancelCauseFunc,
		error,
	) cli.CompletionProvider{
		"returned": func(_ context.CancelCauseFunc, failure error) cli.CompletionProvider {
			return func(context.Context, cli.CompletionRequest) ([]cli.CompletionCandidate, error) {
				return nil, cancellationFailure{cause: context.Canceled, err: failure}
			}
		},
		"observed after return": func(cancel context.CancelCauseFunc, failure error) cli.CompletionProvider {
			return func(context.Context, cli.CompletionRequest) ([]cli.CompletionCandidate, error) {
				cancel(failure)

				return nil, nil
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			secretValue := name + "-completion-cancellation-secret"
			failure := &secretCallbackFailure{value: secretValue}
			ctx, cancel := context.WithCancelCause(context.Background())
			application, err := cli.Compile(cli.NewCommand(
				"tool",
				cli.WithOptions(
					cli.StringOption("token").Secret(),
					cli.StringOption("region").Completion(provider(cancel, failure)),
				),
			))
			if err != nil {
				t.Fatalf("Compile() error = %v", err)
			}

			_, completionErr := application.Complete(ctx, []string{"--region", ""})
			if !errors.Is(completionErr, failure) || !errors.Is(completionErr, context.Canceled) {
				t.Fatalf("Complete() error = %v, want cancellation and cause identity", completionErr)
			}
			assertErrorChainOmits(t, completionErr, secretValue)
			var exposed *secretCallbackFailure
			if errors.As(completionErr, &exposed) {
				t.Fatal("errors.As exposed the secret-bearing completion cancellation cause")
			}
			assertProtectedUnwrap(t, completionErr, secretValue)
		})
	}
}

func TestSecretChildProtectsRootCompletionCancellation(t *testing.T) {
	t.Parallel()

	for name, provider := range map[string]func(
		context.CancelCauseFunc,
		error,
	) cli.CompletionProvider{
		"returned": func(_ context.CancelCauseFunc, failure error) cli.CompletionProvider {
			return func(context.Context, cli.CompletionRequest) ([]cli.CompletionCandidate, error) {
				return nil, cancellationFailure{cause: context.Canceled, err: failure}
			}
		},
		"observed after return": func(cancel context.CancelCauseFunc, failure error) cli.CompletionProvider {
			return func(context.Context, cli.CompletionRequest) ([]cli.CompletionCandidate, error) {
				cancel(failure)

				return nil, nil
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			secretValue := name + "-child-protected-root-completion-cancellation-secret"
			failure := &secretCallbackFailure{value: secretValue}
			ctx, cancel := context.WithCancelCause(context.Background())
			application, err := cli.Compile(cli.NewCommand(
				"tool",
				cli.WithOptions(cli.StringOption("region").Completion(provider(cancel, failure))),
				cli.WithSubcommands(cli.NewCommand(
					"deploy",
					cli.WithOptions(cli.StringOption("token").Secret()),
				)),
			))
			if err != nil {
				t.Fatalf("Compile() error = %v", err)
			}

			_, completionErr := application.Complete(ctx, []string{"--region", ""})
			if !errors.Is(completionErr, failure) || !errors.Is(completionErr, context.Canceled) {
				t.Fatalf("Complete() error = %v, want cancellation and cause identity", completionErr)
			}
			assertErrorChainOmits(t, completionErr, secretValue)
			var exposed *secretCallbackFailure
			if errors.As(completionErr, &exposed) {
				t.Fatal("errors.As exposed the child-protected root completion cause")
			}
			assertProtectedUnwrap(t, completionErr, secretValue)
		})
	}
}

type cancellationFailure struct {
	cause error
	err   error
}

func (failure cancellationFailure) Error() string { return failure.err.Error() }

func (failure cancellationFailure) Is(target error) bool {
	return errors.Is(failure.cause, target) || errors.Is(failure.err, target)
}

type secretCallbackFailure struct {
	value string
}

func (failure *secretCallbackFailure) Error() string {
	return "callback exposed " + failure.value
}

type secretValueFailure struct {
	value string
}

func (failure secretValueFailure) Error() string {
	return "callback exposed " + failure.value
}

type failingSecretWriter struct {
	err error
}

func (writer failingSecretWriter) Write([]byte) (int, error) {
	return 0, writer.err
}

func assertErrorChainOmits(t *testing.T, err error, forbidden string) {
	t.Helper()
	if err == nil {
		return
	}
	if strings.Contains(err.Error(), forbidden) {
		t.Fatalf("error chain contains secret value: %q", err.Error())
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		for _, child := range joined.Unwrap() {
			assertErrorChainOmits(t, child, forbidden)
		}
		return
	}
	if wrapped := errors.Unwrap(err); wrapped != nil {
		assertErrorChainOmits(t, wrapped, forbidden)
	}
}

func assertProtectedUnwrap(t *testing.T, err error, forbidden string) {
	t.Helper()
	cause := errors.Unwrap(err)
	if cause == nil || cause.Error() != "sensitive application error" {
		t.Fatalf("errors.Unwrap() = %v, want safe protected cause", cause)
	}
	if diagnostic := fmt.Sprintf("%#v", cause); strings.Contains(diagnostic, forbidden) {
		t.Fatalf("protected cause diagnostic contains secret value: %q", diagnostic)
	}
	if errors.Unwrap(cause) != nil {
		t.Fatalf("protected cause unwraps to application error: %v", errors.Unwrap(cause))
	}
}

func TestBidiControlsCannotReachCommandSurfaces(t *testing.T) {
	t.Parallel()

	const bidi = "\u202e"
	if _, err := cli.Compile(cli.NewCommand("safe" + bidi + "name")); err == nil {
		t.Fatal("Compile() error = nil, want bidi command name rejection")
	}

	application, err := cli.Compile(cli.NewCommand(
		"tool",
		cli.WithHandler(func(_ context.Context, invocation cli.Invocation) error {
			return invocation.Output().Info("before" + bidi + "after")
		}),
	))
	if err != nil {
		t.Fatalf("compile safe command: %v", err)
	}
	stdout := new(bytes.Buffer)
	result := application.Run(context.Background(), cli.Request{Stdout: stdout})
	if result.Err != nil {
		t.Fatalf("Run() error = %v", result.Err)
	}
	if strings.Contains(stdout.String(), bidi) {
		t.Fatalf("stdout contains bidi control: %q", stdout.String())
	}

	result = application.Run(context.Background(), cli.Request{Args: []string{"--unsafe" + bidi}})
	if result.Err == nil {
		t.Fatal("Run() error = nil, want unknown option")
	}
	if strings.Contains(result.Err.Error(), bidi) {
		t.Fatalf("public error contains bidi control: %q", result.Err.Error())
	}
}
