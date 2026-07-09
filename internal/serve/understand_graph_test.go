package serve

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"reasonix/internal/config"
	"reasonix/internal/control"
	"reasonix/internal/event"
)

func TestUnderstandGraphEndpoint(t *testing.T) {
	// Create a workspace with a knowledge-graph.json
	dir := t.TempDir()
	uaDir := filepath.Join(dir, ".understand-anything")
	if err := os.MkdirAll(uaDir, 0o755); err != nil {
		t.Fatal(err)
	}
	graph := map[string]any{
		"version": "1.0.0",
		"project": map[string]any{
			"name":        "test",
			"languages":   []string{"go"},
			"frameworks":  []string{},
			"description": "A test project",
			"analyzedAt":  "2026-01-01T00:00:00Z",
		},
		"nodes":  []map[string]any{},
		"edges":  []map[string]any{},
		"layers": []map[string]any{},
		"tour":   []map[string]any{},
	}
	data, _ := json.Marshal(graph)
	os.WriteFile(filepath.Join(uaDir, "knowledge-graph.json"), data, 0o644)

	// Create a stub controller that returns our workspace root
	ctrl := &stubWorkspaceController{root: dir}
	s := &Server{ctrl: ctrl, auth: newAuthGate(config.ServeConfig{})}

	req := httptest.NewRequest("GET", "/api/understand/graph", nil)
	w := httptest.NewRecorder()
	s.understandGraph(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var result map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}
	if result["version"] != "1.0.0" {
		t.Errorf("expected version 1.0.0, got %v", result["version"])
	}
}

func TestUnderstandGraphEndpointNoGraph(t *testing.T) {
	dir := t.TempDir()
	ctrl := &stubWorkspaceController{root: dir}
	s := &Server{ctrl: ctrl, auth: newAuthGate(config.ServeConfig{})}

	req := httptest.NewRequest("GET", "/api/understand/graph", nil)
	w := httptest.NewRecorder()
	s.understandGraph(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

// stubWorkspaceController implements just enough of control.SessionAPI for testing.
type stubWorkspaceController struct {
	root string
	control.Controller
}

func (s *stubWorkspaceController) WorkspaceRoot() string { return s.root }
func (s *stubWorkspaceController) SessionDir() string    { return s.root }
func (s *stubWorkspaceController) Sink() event.Sink      { return event.Discard }
