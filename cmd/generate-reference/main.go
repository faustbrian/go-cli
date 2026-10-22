// Command generate-reference reproduces docs/generated from referenceapp.New.
// Run go run ./cmd/generate-reference -out DIR with a trusted operator-owned
// directory. It writes readable Markdown/manifest/shell scripts but executes no
// shell or provider and adds no cgo/service dependency. Writes are sequential
// and non-transactional; failure can leave partially replaced artifacts.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	cli "github.com/faustbrian/go-cli/v2"
	"github.com/faustbrian/go-cli/v2/internal/referenceapp"
)

func main() {
	outputDirectory := flag.String("out", "docs/generated", "output directory")
	flag.Parse()
	if err := generate(*outputDirectory); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func generate(outputDirectory string) error {
	application, err := referenceapp.New()
	if err != nil {
		return fmt.Errorf("compile reference application: %w", err)
	}
	// #nosec G301 -- generated reference directories must be readable.
	if err := os.MkdirAll(outputDirectory, 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	markdown, err := application.Markdown()
	if err != nil {
		return err
	}
	// Repository navigation belongs to this published reference envelope, not
	// the reusable Application.Markdown output for arbitrary consumer commands.
	markdown = strings.Replace(markdown, "\n\n",
		"\n\n[Documentation map](../index.md) · [Repository README](../../README.md)\n"+
			"[Next: Operations and deployment recipes](../operations.md)\n\n", 1)
	manifest, err := application.ManifestJSON()
	if err != nil {
		return err
	}
	artifacts := map[string][]byte{
		"commands.md":   []byte(markdown),
		"manifest.json": manifest,
	}
	for _, shell := range []struct {
		name  string
		shell cli.Shell
	}{
		{name: "tool.bash", shell: cli.ShellBash},
		{name: "_tool", shell: cli.ShellZsh},
		{name: "tool.fish", shell: cli.ShellFish},
		{name: "tool.ps1", shell: cli.ShellPowerShell},
	} {
		completion, completionErr := application.Completion(shell.shell)
		if completionErr != nil {
			return completionErr
		}
		artifacts[shell.name] = []byte(completion)
	}
	for name, contents := range artifacts {
		path := filepath.Join(outputDirectory, name)
		// #nosec G306 -- generated reference artifacts must be readable.
		if err := os.WriteFile(path, contents, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
	}

	return nil
}
