// Package clitest provides invocation-local command captures and testing.TB
// assertions. Start with Run or RunContext and explicit harness options. It uses
// stdlib buffers, not a subprocess, shell or terminal emulator; it introduces no
// service/cgo dependency. Parallel calls isolate argv/streams, not application
// globals, captured callbacks or custom option state; callers synchronize those.
package clitest

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	cli "github.com/faustbrian/go-cli"
)

// Option configures an isolated harness request.
type Option func(*cli.Request)

// WithStdin supplies invocation-local standard input.
func WithStdin(value string) Option {
	return func(request *cli.Request) { request.Stdin = strings.NewReader(value) }
}

// WithOutput selects an invocation-local output policy.
func WithOutput(policy cli.OutputPolicy) Option {
	return func(request *cli.Request) { request.Output = policy }
}

// WithNonInteractive controls the explicit no-interaction policy.
func WithNonInteractive(nonInteractive bool) Option {
	return func(request *cli.Request) { request.NonInteractive = nonInteractive }
}

// Execution captures one isolated in-process command result.
type Execution struct {
	// Result is the framework terminal result; errors do not automatically fail the test.
	Result cli.Result
	// Stdout is captured output from the invocation-local buffer after Run returns.
	Stdout string
	// Stderr is captured diagnostics; custom Options can replace the capture writer.
	Stderr string
}

// Run executes with context.Background and invocation-local streams.
func Run(
	testingContext testing.TB,
	application *cli.Application,
	argv []string,
	options ...Option,
) Execution {
	testingContext.Helper()

	return RunContext(testingContext, context.Background(), application, argv, options...)
}

// RunContext copies argv and supplies private stdout/stderr buffers, then applies
// options in order, ignoring nil. Options can replace any stream or policy;
// custom options must be safe for any concurrent reuse. Execution failures are
// captured, not automatically fatal; use assertions as appropriate. Borrowed
// callbacks must finish their IO before return, and no stream is closed here.
func RunContext(
	testingContext testing.TB,
	ctx context.Context,
	application *cli.Application,
	argv []string,
	options ...Option,
) Execution {
	testingContext.Helper()
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	request := cli.Request{
		Args: append([]string(nil), argv...), Stdout: stdout, Stderr: stderr,
	}
	for _, option := range options {
		if option != nil {
			option(&request)
		}
	}
	result := application.Run(ctx, request)

	return Execution{Result: result, Stdout: stdout.String(), Stderr: stderr.String()}
}

// AssertSuccess requires a nil terminal error and zero exit status.
func (execution Execution) AssertSuccess(testingContext testing.TB) {
	testingContext.Helper()
	if execution.Result.Err != nil || execution.Result.ExitCode != 0 {
		testingContext.Fatalf(
			"execution = error %v, exit %d; want success",
			execution.Result.Err,
			execution.Result.ExitCode,
		)
	}
}

// AssertExitCode requires an exact portable exit status.
func (execution Execution) AssertExitCode(testingContext testing.TB, want int) {
	testingContext.Helper()
	if execution.Result.ExitCode != want {
		testingContext.Fatalf("exit code = %d, want %d", execution.Result.ExitCode, want)
	}
}

// AssertStdout requires exact standard output.
func (execution Execution) AssertStdout(testingContext testing.TB, want string) {
	testingContext.Helper()
	if execution.Stdout != want {
		testingContext.Fatalf("stdout = %q, want %q", execution.Stdout, want)
	}
}

// AssertStderr requires exact standard error.
func (execution Execution) AssertStderr(testingContext testing.TB, want string) {
	testingContext.Helper()
	if execution.Stderr != want {
		testingContext.Fatalf("stderr = %q, want %q", execution.Stderr, want)
	}
}

// AssertCommand requires the selected command's stable name.
func (execution Execution) AssertCommand(testingContext testing.TB, want string) {
	testingContext.Helper()
	if got := execution.Result.Command.Name(); got != want {
		testingContext.Fatalf("selected command = %q, want %q", got, want)
	}
}

// AssertErrorIs requires errors.Is compatibility with target.
func (execution Execution) AssertErrorIs(testingContext testing.TB, target error) {
	testingContext.Helper()
	if !errors.Is(execution.Result.Err, target) {
		testingContext.Fatalf("error = %v, want errors.Is target %v", execution.Result.Err, target)
	}
}
