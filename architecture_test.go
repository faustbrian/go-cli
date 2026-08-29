package cli_test

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"os/exec"
	"strconv"
	"strings"
	"testing"
)

func TestArchitectureKeepsEngineAndRuntimeDetailsInternal(t *testing.T) {
	t.Parallel()

	fset := token.NewFileSet()
	sourceFiles, err := exec.Command("git", "ls-files", "-z", "--cached", "--others", "--exclude-standard", "--", "*.go").Output()
	if err != nil {
		t.Fatalf("enumerate repository source files: %v", err)
	}
	for _, rawPath := range bytes.Split(sourceFiles, []byte{0}) {
		path := string(rawPath)
		if path == "" || strings.HasPrefix(path, "benchmarks/") ||
			strings.HasPrefix(path, ".golib-tooling/") ||
			strings.HasPrefix(path, ".verification/") ||
			strings.HasSuffix(path, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, imported := range file.Imports {
			name, err := strconv.Unquote(imported.Path.Value)
			if err != nil {
				t.Fatalf("unquote import in %s: %v", path, err)
			}
			if strings.HasPrefix(name, "github.com/spf13/") ||
				strings.HasPrefix(name, "github.com/urfave/cli") ||
				strings.HasPrefix(name, "github.com/alecthomas/kong") {
				t.Errorf("%s imports an external parser framework", path)
			}
			if name == "unsafe" || name == "C" || name == "os/exec" || name == "reflect" {
				t.Errorf("%s imports forbidden runtime facility %q", path, name)
			}
		}
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if ok && function.Recv == nil && function.Name.Name == "init" {
				t.Errorf("%s contains package-global init behavior", path)
			}
		}
		for _, comment := range file.Comments {
			if strings.Contains(comment.Text(), "go:linkname") {
				t.Errorf("%s contains forbidden go:linkname directive", path)
			}
		}
	}
}
