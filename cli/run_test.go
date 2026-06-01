package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runArgs invokes Run with the given args and stdin string, capturing stdout,
// stderr, and the exit code.
func runArgs(t *testing.T, args []string, stdin string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errw bytes.Buffer
	code = Run(args, strings.NewReader(stdin), &out, &errw)
	return code, out.String(), errw.String()
}

func TestRunHTMLDefault(t *testing.T) {
	code, out, errw := runArgs(t, nil, "hello *world*")
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, errw)
	}
	if !strings.Contains(out, "<p>hello <strong>world</strong></p>") {
		t.Errorf("unexpected HTML:\n%s", out)
	}
}

func TestRunJSONFormat(t *testing.T) {
	code, out, errw := runArgs(t, []string{"-t", "json"}, "hi")
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, errw)
	}
	var v map[string]any
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}
	if v["tag"] != "doc" {
		t.Errorf("expected top-level tag=doc, got %v", v["tag"])
	}
}

func TestRunASTFormat(t *testing.T) {
	code, out, _ := runArgs(t, []string{"--to", "ast"}, "hi")
	if code != 0 {
		t.Fatalf("exit = %d", code)
	}
	if !strings.HasPrefix(out, "doc\n  para\n") {
		t.Errorf("unexpected AST output:\n%s", out)
	}
}

func TestRunASTSourcepos(t *testing.T) {
	code, out, _ := runArgs(t, []string{"-t", "ast", "-p"}, "hi")
	if code != 0 {
		t.Fatalf("exit = %d", code)
	}
	if !strings.Contains(out, "(1:1:0") {
		t.Errorf("expected source positions in output:\n%s", out)
	}
}

func TestRunJSONSourcepos(t *testing.T) {
	code, out, _ := runArgs(t, []string{"-t", "json", "--sourcepos"}, "hi")
	if code != 0 {
		t.Fatalf("exit = %d", code)
	}
	if !strings.Contains(out, `"pos"`) {
		t.Errorf("expected pos objects in JSON output:\n%s", out)
	}
}

func TestRunVersion(t *testing.T) {
	code, out, _ := runArgs(t, []string{"--version"}, "")
	if code != 0 {
		t.Fatalf("exit = %d", code)
	}
	if !strings.Contains(out, Version) {
		t.Errorf("version output %q does not contain %q", out, Version)
	}
}

func TestRunUnknownFormat(t *testing.T) {
	code, out, errw := runArgs(t, []string{"-t", "xml"}, "hi")
	if code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
	if out != "" {
		t.Errorf("expected no stdout on error, got %q", out)
	}
	if !strings.Contains(errw, "xml") {
		t.Errorf("stderr should mention the bad format, got %q", errw)
	}
}

func TestRunUnknownFlag(t *testing.T) {
	code, _, errw := runArgs(t, []string{"--bogus"}, "hi")
	if code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
	if errw == "" {
		t.Errorf("expected a usage message on stderr")
	}
}

func TestRunHelp(t *testing.T) {
	code, _, errw := runArgs(t, []string{"-h"}, "")
	if code != 0 {
		t.Errorf("exit = %d, want 0 for help", code)
	}
	if !strings.Contains(errw, "Usage") && !strings.Contains(errw, "usage") {
		t.Errorf("expected usage text, got %q", errw)
	}
}

func TestRunFileInput(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "doc.dj")
	if err := os.WriteFile(path, []byte("from file"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out, errw := runArgs(t, []string{path}, "from stdin")
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, errw)
	}
	if !strings.Contains(out, "from file") {
		t.Errorf("expected file content, got:\n%s", out)
	}
	if strings.Contains(out, "from stdin") {
		t.Errorf("stdin should be ignored when files are given:\n%s", out)
	}
}

func TestRunMultipleFilesConcatenated(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.dj")
	b := filepath.Join(dir, "b.dj")
	os.WriteFile(a, []byte("# First\n"), 0o644)
	os.WriteFile(b, []byte("# Second\n"), 0o644)
	code, out, errw := runArgs(t, []string{a, b}, "")
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, errw)
	}
	if !strings.Contains(out, "First") || !strings.Contains(out, "Second") {
		t.Errorf("expected both files rendered, got:\n%s", out)
	}
}

func TestRunOutputToFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.html")
	code, out, errw := runArgs(t, []string{"-o", path}, "hi")
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, errw)
	}
	if out != "" {
		t.Errorf("expected empty stdout when -o is set, got %q", out)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "<p>hi</p>") {
		t.Errorf("output file missing rendered HTML:\n%s", data)
	}
}

func TestRunMissingFile(t *testing.T) {
	code, out, errw := runArgs(t, []string{"/no/such/file.dj"}, "")
	if code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
	if out != "" {
		t.Errorf("expected no stdout on read error, got %q", out)
	}
	if !strings.HasPrefix(errw, "djot:") {
		t.Errorf("expected error prefixed with djot:, got %q", errw)
	}
}
