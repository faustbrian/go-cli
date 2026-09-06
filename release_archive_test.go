package cli_test

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestReleaseArchiveContainsStandaloneModule(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	repository := t.TempDir()
	script, err := os.ReadFile("scripts/build-release.sh")
	if err != nil {
		t.Fatalf("read release script: %v", err)
	}
	if err := os.Mkdir(filepath.Join(repository, "scripts"), 0o755); err != nil {
		t.Fatalf("create scripts directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repository, "scripts", "build-release.sh"), script, 0o755); err != nil {
		t.Fatalf("write release script: %v", err)
	}
	writeReleaseFixture(t, repository, "README.md", "tagged source\n")
	writeReleaseFixture(t, repository, "go.mod", "module example.com/cli\n\ngo 1.26.6\n")
	runGit(t, repository, "init")
	runGit(t, repository, "config", "user.email", "release-test@example.com")
	runGit(t, repository, "config", "user.name", "Release Test")
	runGit(t, repository, "add", "README.md", "go.mod", "scripts/build-release.sh")
	runGit(t, repository, "commit", "-m", "tagged source")
	runGit(t, repository, "tag", "-a", "v1.2.3", "-m", "v1.2.3")
	writeReleaseFixture(t, repository, "README.md", "later head\n")
	writeReleaseFixture(t, repository, "after-tag.txt", "must not be archived\n")
	runGit(t, repository, "add", "README.md", "after-tag.txt")
	runGit(t, repository, "commit", "-m", "later head")

	output := filepath.Join(t.TempDir(), "artifacts")
	command := exec.CommandContext(ctx, filepath.Join(repository, "scripts", "build-release.sh"), "v1.2.3", output)
	command.Dir = repository
	if combined, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build release archive: %v: %s", err, combined)
	}

	archive, err := os.Open(filepath.Join(output, "cli-v1.2.3.tar.gz"))
	if err != nil {
		t.Fatalf("open release archive: %v", err)
	}
	defer func() {
		if err := archive.Close(); err != nil {
			t.Errorf("close release archive: %v", err)
		}
	}()

	compressed, err := gzip.NewReader(archive)
	if err != nil {
		t.Fatalf("open compressed release archive: %v", err)
	}
	defer func() {
		if err := compressed.Close(); err != nil {
			t.Errorf("close compressed release archive: %v", err)
		}
	}()

	want := map[string]string{
		"cli-v1.2.3/README.md": "tagged source\n",
		"cli-v1.2.3/go.mod":    "module example.com/cli\n\ngo 1.26.6\n",
	}
	found := make(map[string]bool, len(want))
	reader := tar.NewReader(compressed)
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("read release archive: %v", err)
		}
		if header.Name == "cli-v1.2.3/after-tag.txt" {
			t.Error("release archive contains a file committed after the release tag")
		}
		content, ok := want[header.Name]
		if !ok {
			continue
		}
		actual, err := io.ReadAll(reader)
		if err != nil {
			t.Fatalf("read %s: %v", header.Name, err)
		}
		if string(actual) != content {
			t.Errorf("%s content = %q, want %q", header.Name, actual, content)
		}
		found[header.Name] = true
	}
	for path := range want {
		if !found[path] {
			t.Errorf("release archive does not contain %s", path)
		}
	}
}

func writeReleaseFixture(t *testing.T, repository, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(repository, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func runGit(t *testing.T, repository string, arguments ...string) {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", repository}, arguments...)...)
	if combined, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", arguments, err, combined)
	}
}
