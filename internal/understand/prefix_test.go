package understand

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrefixNoGraph(t *testing.T) {
	dir := t.TempDir()
	if got := Prefix(dir); got != "" {
		t.Errorf("Prefix() = %q, want empty string when no graph exists", got)
	}
}

func TestPrefixEmptyWorkspace(t *testing.T) {
	if got := Prefix(""); got != "" {
		t.Errorf("Prefix(\"\") = %q, want empty string", got)
	}
}

func TestPrefixDeterministic(t *testing.T) {
	dir := t.TempDir()
	writeTestGraph(t, dir)

	first := Prefix(dir)
	second := Prefix(dir)

	if first != second {
		t.Errorf("Prefix() not deterministic:\nfirst  = %q\nsecond = %q", first, second)
	}
}

func TestPrefixByteIdentical(t *testing.T) {
	dir := t.TempDir()
	writeTestGraph(t, dir)

	first := []byte(Prefix(dir))
	second := []byte(Prefix(dir))

	if len(first) != len(second) {
		t.Fatalf("byte lengths differ: %d vs %d", len(first), len(second))
	}
	for i := range first {
		if first[i] != second[i] {
			t.Fatalf("byte %d differs: %d vs %d\nfirst  = %q\nsecond = %q", i, first[i], second[i], string(first), string(second))
		}
	}
}

func TestPrefixContainsProjectInfo(t *testing.T) {
	dir := t.TempDir()
	writeTestGraph(t, dir)

	got := Prefix(dir)
	if got == "" {
		t.Fatal("expected non-empty prefix")
	}
	if !strings.Contains(got, "test-project") {
		t.Errorf("expected project name in prefix, got: %s", got)
	}
	if !strings.Contains(got, "API Layer") {
		t.Errorf("expected layer name in prefix, got: %s", got)
	}
	if !strings.Contains(got, "Authentication") {
		t.Errorf("expected module name in prefix, got: %s", got)
	}
	if !strings.Contains(got, "commit:") {
		t.Errorf("expected commit hash in prefix, got: %s", got)
	}
	// Must NOT contain analyzedAt timestamp
	if strings.Contains(got, "analyzedAt") || strings.Contains(got, "2026-") {
		t.Errorf("prefix contains timestamp — cache-stable guarantee broken: %s", got)
	}
}

func TestPrefixMaxLength(t *testing.T) {
	dir := t.TempDir()
	// Build a graph with many layers
	g := largeTestGraph()
	writeGraph(t, dir, g)

	got := Prefix(dir)
	if len(got) > 2000 {
		t.Errorf("Prefix length = %d, want <= 2000", len(got))
	}
}

// --- helpers ---

func writeTestGraph(t *testing.T, dir string) {
	t.Helper()
	g := map[string]any{
		"version": "1.0.0",
		"project": map[string]any{
			"name":          "test-project",
			"languages":     []string{"go", "typescript"},
			"frameworks":    []string{"react", "wails"},
			"description":   "A test project for understand prefix",
			"gitCommitHash": "abc123def4567890",
		},
		"nodes": []map[string]any{
			{
				"id": "file:main.go", "type": "file", "name": "main.go",
				"filePath": "main.go", "summary": "Entry point",
				"tags": []string{"entrypoint"}, "complexity": "moderate",
			},
			{
				"id": "file:auth/login.go", "type": "file", "name": "login.go",
				"filePath": "auth/login.go", "summary": "Handles login",
				"tags": []string{"auth"}, "complexity": "moderate",
			},
			{
				"id": "module:auth", "type": "module", "name": "Authentication",
				"summary": "Authentication module",
				"tags":    []string{"auth", "security"}, "complexity": "moderate",
			},
			{
				"id": "function:main.go:main", "type": "function", "name": "main",
				"filePath": "main.go", "summary": "Application entry",
				"tags": []string{"entrypoint"}, "complexity": "simple",
			},
		},
		"edges": []map[string]any{
			{"source": "file:main.go", "target": "module:auth", "type": "imports"},
		},
		"layers": []map[string]any{
			{
				"id": "layer-api", "name": "API Layer",
				"description": "HTTP handlers and routing",
				"nodeIds":     []string{"file:main.go"},
			},
			{
				"id": "layer-auth", "name": "Auth Layer",
				"description": "Authentication and authorization",
				"nodeIds":     []string{"file:auth/login.go", "module:auth"},
			},
		},
	}
	writeGraph(t, dir, g)
}

func writeGraph(t *testing.T, dir string, g map[string]any) {
	t.Helper()
	uaDir := filepath.Join(dir, ".understand-anything")
	if err := os.MkdirAll(uaDir, 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := json.MarshalIndent(g, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(uaDir, "knowledge-graph.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func largeTestGraph() map[string]any {
	nodes := make([]map[string]any, 0, 5000)
	for i := 0; i < 5000; i++ {
		nodes = append(nodes, map[string]any{
			"id":      "node:" + string(rune('a'+i%26)) + string(rune('0'+i%10)),
			"type":    "function",
			"name":    "node",
			"summary": "test node",
			"tags":    []string{"test"},
		})
	}
	layers := make([]map[string]any, 0, 50)
	for i := 0; i < 50; i++ {
		layers = append(layers, map[string]any{
			"id":          "layer-" + string(rune('a'+i)),
			"name":        "Layer " + string(rune('A'+i)),
			"description": "Description for layer " + string(rune('a'+i)),
			"nodeIds":     []string{},
		})
	}
	return map[string]any{
		"version": "1.0.0",
		"project": map[string]any{
			"name":          "large-project",
			"languages":     []string{"go"},
			"description":   "A very large project",
			"gitCommitHash": "abcdef1234567890",
		},
		"nodes":  nodes,
		"edges":  []map[string]any{},
		"layers": layers,
	}
}
