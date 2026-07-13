package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// FavoriteItem represents a single bookmarked message.
type FavoriteItem struct {
	ID                string `json:"id"`
	Text              string `json:"text"`
	Source            string `json:"source"` // "user" or "assistant"
	OriginalMessageID string `json:"originalMessageId"`
	CreatedAt         int64  `json:"createdAt"`
	Order             int    `json:"order"`
}

// FavoritesView is the wire format sent to the frontend.
type FavoritesView struct {
	Items []FavoriteItem `json:"items"`
}

func favoritesPath(workspaceRoot string) string {
	return filepath.Join(workspaceRoot, ".reasonix", "favorites.json")
}

// GenerateFavoriteID returns a unique ID for a new favorite item.
func (a *App) GenerateFavoriteID() string {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("fav_%d", time.Now().UnixNano())
	}
	return "fav_" + hex.EncodeToString(b[:])
}

// GetFavorites reads the favorites file for a workspace.
// Returns an empty list if the file does not exist or cannot be parsed.
func (a *App) GetFavorites(workspaceRoot string) FavoritesView {
	workspaceRoot = strings.TrimSpace(workspaceRoot)
	if workspaceRoot == "" {
		return FavoritesView{Items: []FavoriteItem{}}
	}

	path := favoritesPath(workspaceRoot)
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			slog.Warn("GetFavorites: read error", "err", err)
		}
		return FavoritesView{Items: []FavoriteItem{}}
	}

	var view FavoritesView
	if err := json.Unmarshal(data, &view); err != nil {
		slog.Warn("GetFavorites: parse error", "err", err)
		return FavoritesView{Items: []FavoriteItem{}}
	}

	if view.Items == nil {
		view.Items = []FavoriteItem{}
	}
	return view
}

// SaveFavorites atomically writes the favorites file for a workspace.
func (a *App) SaveFavorites(workspaceRoot string, view FavoritesView) error {
	workspaceRoot = strings.TrimSpace(workspaceRoot)
	if workspaceRoot == "" {
		return fmt.Errorf("workspace root is empty")
	}

	if view.Items == nil {
		view.Items = []FavoriteItem{}
	}

	// Ensure .reasonix directory exists.
	dir := filepath.Join(workspaceRoot, ".reasonix")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create .reasonix dir: %w", err)
	}

	data, err := json.MarshalIndent(view, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal favorites: %w", err)
	}

	path := favoritesPath(workspaceRoot)
	tmp := path + ".tmp"

	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}

	if err := os.Rename(tmp, path); err != nil {
		// Clean up temp file on failure.
		os.Remove(tmp)
		return fmt.Errorf("rename temp to final: %w", err)
	}

	return nil
}
