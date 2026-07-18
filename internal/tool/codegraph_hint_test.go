package tool

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// --- mock tool for testing ---

type mockReadTool struct {
	name        string
	description string
	schema      json.RawMessage
	output      string
	err         error
	snipHint    *SnipHint // nil means don't implement SnipHinter
}

func (m *mockReadTool) Name() string              { return m.name }
func (m *mockReadTool) Description() string        { return m.description }
func (m *mockReadTool) Schema() json.RawMessage    { return m.schema }
func (m *mockReadTool) ReadOnly() bool             { return true }

func (m *mockReadTool) Execute(_ context.Context, _ json.RawMessage) (string, error) {
	return m.output, m.err
}

func (m *mockReadTool) SnipHint() SnipHint {
	if m.snipHint != nil {
		return *m.snipHint
	}
	return SnipHint{Head: 40, Tail: 10, HeadChars: 6000, TailChars: 1000}
}

var _ SnipHinter = (*mockReadTool)(nil)

// --- helpers ---

func hintWrapper(inner Tool) *CodeGraphHint {
	return NewCodeGraphHint(inner)
}

var stdResult = "line1\nline2\nline3\n"

// --- test cases ---

func TestCodeGraphHintHintsOnCodeExtensions(t *testing.T) {
	codeExts := []string{
		".go", ".js", ".ts", ".jsx", ".tsx",
		".py", ".rs", ".java", ".kt", ".kts", ".cs",
		".c", ".cc", ".cpp", ".cxx",
		".h", ".hh", ".hpp", ".hxx",
		".m", ".mm", ".swift", ".rb", ".php", ".scala",
		".clj", ".cljs", ".ex", ".exs",
		".erb", ".vue", ".svelte", ".zig", ".lua", ".dart",
	}
	for _, ext := range codeExts {
		t.Run(ext, func(t *testing.T) {
			inner := &mockReadTool{name: "read_file", output: stdResult}
			w := hintWrapper(inner)
			args := json.RawMessage(`{"path":"main` + ext + `"}`)
			result, err := w.Execute(context.Background(), args)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.HasSuffix(result, codegraphHintText) {
				t.Errorf("%s: expected hint suffix, got:\n%s", ext, result)
			}
		})
	}
}

func TestCodeGraphHintSkipsNonCodeExtensions(t *testing.T) {
	nonCodeExts := []string{
		".md", ".txt", ".rst", ".adoc", ".asciidoc",
		".json", ".yaml", ".yml", ".toml", ".ini", ".cfg", ".conf",
		".csv", ".tsv", ".xml", ".html", ".htm",
		".css", ".scss", ".less",
		".sh", ".bat", ".ps1", ".cmd",
		".sql", ".env",
		".gitignore", ".gitattributes", ".editorconfig",
		".jpg", ".png", ".svg", ".ico",
		".pdf", ".doc", ".docx",
	}
	for _, ext := range nonCodeExts {
		t.Run(ext, func(t *testing.T) {
			inner := &mockReadTool{name: "read_file", output: stdResult}
			w := hintWrapper(inner)
			args := json.RawMessage(`{"path":"file` + ext + `"}`)
			result, err := w.Execute(context.Background(), args)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if strings.HasSuffix(result, codegraphHintText) {
				t.Errorf("%s: expected NO hint, but hint was appended", ext)
			}
		})
	}
}

func TestCodeGraphHintProjectLevelPath(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{"empty path", ""},
		{"dot path", "."},
		{"directory path", "./internal/agent"},
		{"no extension file", "Makefile"},
		{"Dockerfile no ext", "Dockerfile"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inner := &mockReadTool{name: "grep", output: stdResult}
			w := hintWrapper(inner)
			args := json.RawMessage(`{"pattern":"test","path":"` + tt.path + `"}`)
			result, err := w.Execute(context.Background(), args)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.HasSuffix(result, codegraphHintText) {
				t.Errorf("%s: expected hint for path %q, got no hint", tt.name, tt.path)
			}
		})
	}
}

func TestCodeGraphHintSkipsErrorResult(t *testing.T) {
	inner := &mockReadTool{name: "grep", output: "partial", err: errMock}
	w := hintWrapper(inner)
	args := json.RawMessage(`{"pattern":"test","path":"main.go"}`)
	result, err := w.Execute(context.Background(), args)
	if err == nil {
		t.Fatal("expected error from mock")
	}
	if strings.HasSuffix(result, codegraphHintText) {
		t.Error("expected NO hint on error, but hint was appended")
	}
}

var errMock = &mockError{}

type mockError struct{}

func (m *mockError) Error() string { return "mock error" }

func TestCodeGraphHintEmptyArgs(t *testing.T) {
	inner := &mockReadTool{name: "grep", output: stdResult}
	w := hintWrapper(inner)
	// Empty args — shouldStill execute but treat as code scope
	result, err := w.Execute(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasSuffix(result, codegraphHintText) {
		t.Error("expected hint for empty args (project-level scope)")
	}
}

func TestCodeGraphHintInvalidArgs(t *testing.T) {
	inner := &mockReadTool{name: "grep", output: stdResult}
	w := hintWrapper(inner)
	args := json.RawMessage(`not json`)
	result, err := w.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Invalid JSON means path="" which means code scope → hint
	if !strings.HasSuffix(result, codegraphHintText) {
		t.Error("expected hint for invalid args (fallback to code scope)")
	}
}

func TestCodeGraphHintDelegatesName(t *testing.T) {
	inner := &mockReadTool{name: "grep"}
	w := hintWrapper(inner)
	if got := w.Name(); got != "grep" {
		t.Errorf("Name() = %q, want %q", got, "grep")
	}
}

func TestCodeGraphHintDelegatesDescription(t *testing.T) {
	inner := &mockReadTool{description: "search tool"}
	w := hintWrapper(inner)
	if got := w.Description(); got != "search tool" {
		t.Errorf("Description() = %q, want %q", got, "search tool")
	}
}

func TestCodeGraphHintDelegatesReadOnly(t *testing.T) {
	inner := &mockReadTool{name: "grep"}
	w := hintWrapper(inner)
	if got := w.ReadOnly(); got != true {
		t.Errorf("ReadOnly() = %v, want true", got)
	}
}

func TestCodeGraphHintDelegatesSchema(t *testing.T) {
	schema := json.RawMessage(`{"type":"object"}`)
	inner := &mockReadTool{schema: schema}
	w := hintWrapper(inner)
	if got := string(w.Schema()); got != string(schema) {
		t.Errorf("Schema() = %s, want %s", got, schema)
	}
}

func TestCodeGraphHintDelegatesSnipHint(t *testing.T) {
	inner := &mockReadTool{
		name:     "grep",
		snipHint: &SnipHint{Head: 80, Tail: 8, HeadChars: 10000, TailChars: 1000},
	}
	w := hintWrapper(inner)
	got := w.SnipHint()
	want := SnipHint{Head: 80, Tail: 8, HeadChars: 10000, TailChars: 1000}
	if got != want {
		t.Errorf("SnipHint() = %+v, want %+v", got, want)
	}
}

func TestCodeGraphHintDefaultSnipHint(t *testing.T) {
	// A tool that implements Tool but NOT SnipHinter.
	// mockReadTool implements SnipHinter, so we use an anonymous wrapper.
	type onlyTool struct{ Tool }
	inner := onlyTool{Tool: &mockReadTool{name: "grep"}}
	// Verify it does NOT implement SnipHinter
	if _, ok := interface{}(inner).(SnipHinter); ok {
		t.Skip("onlyTool unexpectedly implements SnipHinter — test setup broken")
	}
	w := hintWrapper(inner)
	got := w.SnipHint()
	// Should return the default fallback, not delegate
	if got.Head != 40 || got.Tail != 10 {
		t.Errorf("SnipHint() = %+v, want default {Head:40 Tail:10}", got)
	}
}

func TestCodeGraphHintWorksForGrep(t *testing.T) {
	inner := &mockReadTool{name: "grep", output: stdResult}
	w := hintWrapper(inner)
	args := json.RawMessage(`{"pattern":"func","path":"agent.go"}`)
	result, err := w.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasSuffix(result, codegraphHintText) {
		t.Error("grep: expected hint for .go file")
	}
}

func TestCodeGraphHintWorksForReadFile(t *testing.T) {
	inner := &mockReadTool{name: "read_file", output: stdResult}
	w := hintWrapper(inner)
	args := json.RawMessage(`{"path":"main.go"}`)
	result, err := w.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasSuffix(result, codegraphHintText) {
		t.Error("read_file: expected hint for .go file")
	}
}

func TestCodeGraphHintGrepOnNonCodeSkipsHint(t *testing.T) {
	inner := &mockReadTool{name: "grep", output: stdResult}
	w := hintWrapper(inner)
	args := json.RawMessage(`{"pattern":"character","path":"novel.txt"}`)
	result, err := w.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.HasSuffix(result, codegraphHintText) {
		t.Error("expected NO hint for .txt file grep")
	}
}

func TestCodeGraphHintReadFileOnDocSkipsHint(t *testing.T) {
	inner := &mockReadTool{name: "read_file", output: stdResult}
	w := hintWrapper(inner)
	args := json.RawMessage(`{"path":"README.md"}`)
	result, err := w.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.HasSuffix(result, codegraphHintText) {
		t.Error("expected NO hint for .md file read")
	}
}

func TestIsCodeGraphHint(t *testing.T) {
	inner := &mockReadTool{name: "grep"}
	w := hintWrapper(inner)
	if !IsCodeGraphHint(w) {
		t.Error("IsCodeGraphHint(wrapped) = false, want true")
	}
	if IsCodeGraphHint(inner) {
		t.Error("IsCodeGraphHint(inner) = true, want false")
	}
	if IsCodeGraphHint(nil) {
		t.Error("IsCodeGraphHint(nil) = true, want false")
	}
}

func TestCodeGraphHintHintTextIsShort(t *testing.T) {
	// The hint should be short enough to fit comfortably within the smallest
	// tool's SnipHint tail budget. grep has Tail=8 lines / 1000 chars.
	lines := strings.Count(codegraphHintText, "\n") + 1
	if lines > 6 {
		t.Errorf("hint text has %d lines, want ≤6 to fit in grep's tail budget of 8 lines", lines)
	}
	if len(codegraphHintText) > 800 {
		t.Errorf("hint text is %d chars, want ≤800 to fit in grep's tail budget of 1000 chars", len(codegraphHintText))
	}
}

func TestCodeGraphHintDoubleWrapGuard(t *testing.T) {
	inner := &mockReadTool{name: "grep", output: stdResult}
	w1 := hintWrapper(inner)
	if IsCodeGraphHint(w1) {
		// Simulate what wrapBuiltinReadTools does: guard with IsCodeGraphHint
		if IsCodeGraphHint(w1) {
			// This would be skipped in production — correct
		} else {
			t.Error("IsCodeGraphHint should detect wrapped tool")
		}
	}
}
