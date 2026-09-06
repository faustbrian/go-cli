package clitest_test

import (
	"context"
	"testing"

	cli "github.com/faustbrian/go-cli"
	"github.com/faustbrian/go-cli/clitest"
)

func TestExampleRun(t *testing.T) {
	t.Parallel()

	application, err := cli.Compile(cli.NewCommand(
		"tool",
		cli.WithSubcommands(cli.NewCommand(
			"status",
			cli.WithHandler(func(_ context.Context, invocation cli.Invocation) error {
				return invocation.Output().SetData("ready")
			}),
		)),
	))
	if err != nil {
		t.Fatalf("compile command: %v", err)
	}

	execution := clitest.Run(t, application, []string{"status"})
	execution.AssertSuccess(t)
	execution.AssertCommand(t, "status")
	execution.AssertStdout(t, "ready\n")
	execution.AssertStderr(t, "")
}
