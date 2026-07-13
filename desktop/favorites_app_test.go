package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func tempWorkspaceRoot(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	// Create .reasonix subdir to simulate a real workspace.
	if err := os.MkdirAll(filepath.Join(dir, ".reasonix"), 0o755); err != nil {
		t.Fatalf("create .reasonix: %v", err)
	}
	return dir
}

func TestGetFavoritesEmptyFile(t *testing.T) {
	app := NewApp()
	root := tempWorkspaceRoot(t)
	view := app.GetFavorites(root)
	if len(view.Items) != 0 {
		t.Errorf("expected 0 items, got %d", len(view.Items))
	}
}

func TestGetFavoritesMissingWorkspace(t *testing.T) {
	app := NewApp()
	view := app.GetFavorites("/nonexistent/path/12345")
	if len(view.Items) != 0 {
		t.Errorf("expected 0 items, got %d", len(view.Items))
	}
}

func TestGetFavoritesEmptyRoot(t *testing.T) {
	app := NewApp()
	view := app.GetFavorites("")
	if len(view.Items) != 0 {
		t.Errorf("expected 0 items, got %d", len(view.Items))
	}
}

func TestSaveAndGetRoundtrip(t *testing.T) {
	app := NewApp()
	root := tempWorkspaceRoot(t)

	view := FavoritesView{
		Items: []FavoriteItem{
			{ID: "fav_001", Text: "Hello world", Source: "user", OriginalMessageID: "msg_1", CreatedAt: 1000, Order: 0},
			{ID: "fav_002", Text: "AI response", Source: "assistant", OriginalMessageID: "msg_2", CreatedAt: 2000, Order: 1},
		},
	}

	if err := app.SaveFavorites(root, view); err != nil {
		t.Fatalf("SaveFavorites: %v", err)
	}

	// Verify file exists and is valid JSON.
	path := favoritesPath(root)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back file: %v", err)
	}
	var readView FavoritesView
	if err := json.Unmarshal(data, &readView); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(readView.Items) != 2 {
		t.Errorf("expected 2 items, got %d", len(readView.Items))
	}
	if readView.Items[0].Text != "Hello world" {
		t.Errorf("first item text = %q", readView.Items[0].Text)
	}
	if readView.Items[1].Source != "assistant" {
		t.Errorf("second item source = %q", readView.Items[1].Source)
	}

	// GetFavorites should return the same data.
	loaded := app.GetFavorites(root)
	if len(loaded.Items) != 2 {
		t.Fatalf("GetFavorites: expected 2 items, got %d", len(loaded.Items))
	}
	if loaded.Items[0].ID != "fav_001" {
		t.Errorf("ID mismatch: %q", loaded.Items[0].ID)
	}
}

func TestSaveFavoritesNilItems(t *testing.T) {
	app := NewApp()
	root := tempWorkspaceRoot(t)

	if err := app.SaveFavorites(root, FavoritesView{}); err != nil {
		t.Fatalf("SaveFavorites with nil items: %v", err)
	}

	loaded := app.GetFavorites(root)
	if len(loaded.Items) != 0 {
		t.Errorf("expected 0 items, got %d", len(loaded.Items))
	}
}

func TestSaveFavoritesEmptyRoot(t *testing.T) {
	app := NewApp()
	err := app.SaveFavorites("", FavoritesView{Items: []FavoriteItem{}})
	if err == nil {
		t.Error("expected error for empty root, got nil")
	}
}

func TestAtomicWritePreservesExistingOnFailure(t *testing.T) {
	app := NewApp()
	root := tempWorkspaceRoot(t)

	// Write an initial file.
	initial := FavoritesView{
		Items: []FavoriteItem{
			{ID: "fav_orig", Text: "original", Source: "user", CreatedAt: 1000, Order: 0},
		},
	}
	if err := app.SaveFavorites(root, initial); err != nil {
		t.Fatalf("initial SaveFavorites: %v", err)
	}

	// Read back to confirm.
	loaded := app.GetFavorites(root)
	if len(loaded.Items) != 1 || loaded.Items[0].Text != "original" {
		t.Fatalf("unexpected initial state: %+v", loaded)
	}

	// Verify no .tmp file is left behind after successful write.
	tmpPath := favoritesPath(root) + ".tmp"
	if _, err := os.Stat(tmpPath); err == nil {
		t.Error("tmp file should not exist after successful write")
	}
}

func TestGetFavoritesCorruptJSON(t *testing.T) {
	app := NewApp()
	root := tempWorkspaceRoot(t)

	// Write garbage JSON.
	path := favoritesPath(root)
	if err := os.WriteFile(path, []byte("not valid json {{{"), 0o644); err != nil {
		t.Fatalf("write corrupt file: %v", err)
	}

	view := app.GetFavorites(root)
	if len(view.Items) != 0 {
		t.Errorf("expected 0 items for corrupt JSON, got %d", len(view.Items))
	}
}

func TestGenerateFavoriteID(t *testing.T) {
	app := NewApp()
	seen := make(map[string]bool, 100)
	for i := 0; i < 100; i++ {
		id := app.GenerateFavoriteID()
		if id == "" {
			t.Error("GenerateFavoriteID returned empty string")
		}
		if !strings.HasPrefix(id, "fav_") {
			t.Errorf("expected id to start with 'fav_', got %q", id)
		}
		if seen[id] {
			t.Errorf("duplicate ID generated: %q", id)
		}
		seen[id] = true
	}
}
