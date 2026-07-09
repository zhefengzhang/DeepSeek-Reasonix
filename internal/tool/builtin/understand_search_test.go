package builtin

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"reasonix/internal/tool"
)

func TestUnderstandSearchRegistered(t *testing.T) {
	tl, ok := tool.LookupBuiltin("understand_search")
	if !ok {
		t.Fatal("understand_search not registered")
	}
	if tl.Name() != "understand_search" {
		t.Errorf("Name() = %q, want understand_search", tl.Name())
	}
	if !tl.ReadOnly() {
		t.Error("understand_search must be ReadOnly")
	}
}

func TestUnderstandSearch_Search(t *testing.T) {
	dir := t.TempDir()
	writeTestGraph(t, dir)

	u := understandSearch{workDir: dir}

	out := runTool(t, u, map[string]any{
		"action": "search",
		"query":  "auth",
	})
	if !strings.Contains(out, "auth") && !strings.Contains(out, "Auth") {
		t.Errorf("expected auth in search results, got: %s", out)
	}
}

func TestUnderstandSearch_SearchByType(t *testing.T) {
	dir := t.TempDir()
	writeTestGraph(t, dir)

	u := understandSearch{workDir: dir}

	out := runTool(t, u, map[string]any{
		"action": "search",
		"query":  "server",
		"types":  []string{"file"},
	})
	if !strings.Contains(out, "main.go") {
		t.Errorf("expected main.go in file-type search results, got: %s", out)
	}
}

func TestUnderstandSearch_Outline(t *testing.T) {
	dir := t.TempDir()
	writeTestGraph(t, dir)

	u := understandSearch{workDir: dir}

	out := runTool(t, u, map[string]any{
		"action": "outline",
	})
	if !strings.Contains(out, "test-project") {
		t.Errorf("expected project name in outline, got: %s", out)
	}
	if !strings.Contains(out, "API Layer") {
		t.Errorf("expected layer in outline, got: %s", out)
	}
	if !strings.Contains(out, "main.go") {
		t.Errorf("expected main.go in outline, got: %s", out)
	}
}

func TestUnderstandSearch_Node(t *testing.T) {
	dir := t.TempDir()
	writeTestGraph(t, dir)

	u := understandSearch{workDir: dir}

	out := runTool(t, u, map[string]any{
		"action":  "node",
		"node_id": "module:auth",
	})
	if !strings.Contains(out, "Authentication") || !strings.Contains(out, "Auth") {
		t.Errorf("expected Auth module details, got: %s", out)
	}
	if !strings.Contains(out, "Incoming edges") {
		t.Errorf("expected incoming edges section, got: %s", out)
	}
}

func TestUnderstandSearch_NodeNotFound(t *testing.T) {
	dir := t.TempDir()
	writeTestGraph(t, dir)

	u := understandSearch{workDir: dir}

	_, err := u.Execute(t.Context(), argsJSON(t, map[string]any{
		"action":  "node",
		"node_id": "nonexistent:node",
	}))
	if err == nil {
		t.Error("expected error for nonexistent node")
	}
}

func TestUnderstandSearch_NoGraph(t *testing.T) {
	dir := t.TempDir()
	u := understandSearch{workDir: dir}

	_, err := u.Execute(t.Context(), argsJSON(t, map[string]any{
		"action": "search",
		"query":  "anything",
	}))
	if err == nil {
		t.Error("expected error when no graph exists")
	}
	if !strings.Contains(err.Error(), "run /understand first") {
		t.Errorf("error should mention /understand, got: %v", err)
	}
}

func TestUnderstandSearch_Dashboard(t *testing.T) {
	dir := t.TempDir()
	writeTestGraph(t, dir)

	u := understandSearch{workDir: dir}

	out := runTool(t, u, map[string]any{
		"action": "dashboard",
	})
	if !strings.Contains(out, "pnpm dev") {
		t.Errorf("dashboard should mention pnpm dev, got: %s", out)
	}
}

func TestUnderstandSearch_EmptySearch(t *testing.T) {
	dir := t.TempDir()
	writeTestGraph(t, dir)

	u := understandSearch{workDir: dir}

	_, err := u.Execute(t.Context(), argsJSON(t, map[string]any{
		"action": "search",
		"query":  "",
	}))
	if err == nil {
		t.Error("expected error for empty query")
	}
}

func TestUnderstandSearch_InvalidAction(t *testing.T) {
	dir := t.TempDir()
	u := understandSearch{workDir: dir}

	_, err := u.Execute(t.Context(), argsJSON(t, map[string]any{
		"action": "invalid",
	}))
	if err == nil {
		t.Error("expected error for invalid action")
	}
}

// --- helpers ---

func writeTestGraph(t *testing.T, dir string) {
	t.Helper()
	uaDir := filepath.Join(dir, ".understand-anything")
	if err := os.MkdirAll(uaDir, 0o755); err != nil {
		t.Fatal(err)
	}

	g := map[string]any{
		"version": "1.0.0",
		"project": map[string]any{
			"name":        "test-project",
			"languages":   []string{"go"},
			"frameworks":  []string{},
			"description": "A test project for understand_search",
			"analyzedAt":  "2026-07-09T00:00:00Z",
		},
		"nodes": []map[string]any{
			{
				"id": "file:main.go", "type": "file", "name": "main.go",
				"filePath": "main.go", "summary": "Entry point that starts the HTTP server",
				"tags": []string{"entrypoint", "server"}, "complexity": "moderate",
			},
			{
				"id": "file:auth/login.go", "type": "file", "name": "login.go",
				"filePath": "auth/login.go", "summary": "Handles user login and session creation",
				"tags": []string{"auth", "login"}, "complexity": "moderate",
			},
			{
				"id": "module:auth", "type": "module", "name": "Authentication",
				"filePath": "", "summary": "Authentication module handling login, tokens, and sessions",
				"tags": []string{"auth", "security"}, "complexity": "moderate",
			},
			{
				"id": "function:main.go:main", "type": "function", "name": "main",
				"filePath": "main.go", "lineRange": []int{10, 25},
				"summary": "Application entry point, initializes router and starts server",
				"tags":    []string{"entrypoint"}, "complexity": "simple",
			},
		},
		"edges": []map[string]any{
			{
				"source": "file:main.go", "target": "module:auth", "type": "imports",
				"direction": "forward", "weight": 0.7,
			},
			{
				"source": "function:main.go:main", "target": "module:auth", "type": "calls",
				"direction": "forward", "description": "Initializes auth middleware", "weight": 0.8,
			},
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

	data, err := json.MarshalIndent(g, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(uaDir, "knowledge-graph.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestUnderstandSearch_StalenessWithoutGit(t *testing.T) {
	dir := t.TempDir()
	writeTestGraph(t, dir)
	meta := map[string]any{
		"lastAnalyzedAt": "2026-01-01T00:00:00Z",
		"gitCommitHash":  "0000000000000000000000000000000000000000",
		"version":        "1.0.0",
		"analyzedFiles":  4,
	}
	metaData, _ := json.Marshal(meta)
	os.WriteFile(filepath.Join(dir, ".understand-anything", "meta.json"), metaData, 0o644)

	u := understandSearch{workDir: dir}
	out := runTool(t, u, map[string]any{
		"action": "search",
		"query":  "auth",
	})
	if strings.Contains(out, "Graph may be stale") {
		t.Errorf("unexpected staleness warning outside git repo: %s", out)
	}
}

func TestUnderstandSearch_NoStalenessWhenNoMeta(t *testing.T) {
	dir := t.TempDir()
	writeTestGraph(t, dir)
	u := understandSearch{workDir: dir}
	out := runTool(t, u, map[string]any{
		"action": "search",
		"query":  "auth",
	})
	if strings.Contains(out, "Graph may be stale") {
		t.Errorf("unexpected staleness warning when no meta.json exists: %s", out)
	}
}

func TestUnderstandSearch_StalenessLogic(t *testing.T) {
	dir := t.TempDir()
	writeTestGraph(t, dir)
	u := understandSearch{workDir: dir}

	if note := u.stalenessNote(); note != "" {
		t.Errorf("expected empty note without meta.json, got: %s", note)
	}

	writeMetaJSON(t, dir, uaMeta{LastAnalyzedAt: "2026-01-01", GitCommitHash: "", AnalyzedFiles: 4})
	if note := u.stalenessNote(); note != "" {
		t.Errorf("expected empty note with empty hash, got: %s", note)
	}

	writeMetaJSON(t, dir, uaMeta{LastAnalyzedAt: "2026-01-01", GitCommitHash: "abc123def456", AnalyzedFiles: 4})
	if note := u.stalenessNote(); note != "" {
		t.Errorf("expected empty note when git not available, got: %s", note)
	}
}

func writeMetaJSON(t *testing.T, dir string, m uaMeta) {
	t.Helper()
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, ".understand-anything", "meta.json"), data, 0o644)
}

func TestUnderstandSearch_TraverseForward(t *testing.T) {
	dir := t.TempDir()
	writeTestGraph(t, dir)
	u := understandSearch{workDir: dir}
	out := runTool(t, u, map[string]any{
		"action":    "traverse",
		"from":      "file:main.go",
		"direction": "forward",
	})
	if !strings.Contains(out, "Authentication") {
		t.Errorf("expected auth module in traverse results, got: %s", out)
	}
	if !strings.Contains(out, "imports") {
		t.Errorf("expected 'imports' edge type, got: %s", out)
	}
}

func TestUnderstandSearch_TraverseBackward(t *testing.T) {
	dir := t.TempDir()
	writeTestGraph(t, dir)
	u := understandSearch{workDir: dir}
	out := runTool(t, u, map[string]any{
		"action":    "traverse",
		"from":      "module:auth",
		"direction": "backward",
	})
	if !strings.Contains(out, "main.go") {
		t.Errorf("expected main.go in backward traverse, got: %s", out)
	}
}

func TestUnderstandSearch_TraverseBoth(t *testing.T) {
	dir := t.TempDir()
	writeTestGraph(t, dir)
	u := understandSearch{workDir: dir}
	out := runTool(t, u, map[string]any{
		"action":    "traverse",
		"from":      "module:auth",
		"direction": "both",
	})
	if !strings.Contains(out, "Found") {
		t.Errorf("expected results in both-direction traverse, got: %s", out)
	}
}

func TestUnderstandSearch_TraverseFilterByType(t *testing.T) {
	dir := t.TempDir()
	writeTestGraph(t, dir)
	u := understandSearch{workDir: dir}
	out := runTool(t, u, map[string]any{
		"action":    "traverse",
		"from":      "function:main.go:main",
		"direction": "forward",
		"edge_type": "calls",
	})
	if !strings.Contains(out, "calls") {
		t.Errorf("expected 'calls' edge in filtered traverse, got: %s", out)
	}
	if strings.Contains(out, "imports") {
		t.Errorf("unexpected 'imports' edge in calls-filtered traverse: %s", out)
	}
}

func TestUnderstandSearch_TraverseNodeNotFound(t *testing.T) {
	dir := t.TempDir()
	writeTestGraph(t, dir)
	u := understandSearch{workDir: dir}
	_, err := u.Execute(t.Context(), argsJSON(t, map[string]any{
		"action": "traverse",
		"from":   "nonexistent:node",
	}))
	if err == nil {
		t.Error("expected error for nonexistent start node")
	}
}

func TestUnderstandSearch_TraverseNoGraph(t *testing.T) {
	dir := t.TempDir()
	u := understandSearch{workDir: dir}
	_, err := u.Execute(t.Context(), argsJSON(t, map[string]any{
		"action": "traverse",
		"from":   "module:auth",
	}))
	if err == nil {
		t.Error("expected error when no graph exists")
	}
}
