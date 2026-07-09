package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// UnderstandGraphStatusView is a lightweight summary of the knowledge graph
// state, surfaced to the frontend for a status-bar indicator.
type UnderstandGraphStatusView struct {
	Available bool `json:"available"`
	Stale     bool `json:"stale"`
	NodeCount int  `json:"nodeCount"`
	EdgeCount int  `json:"edgeCount"`
}

// understandGraphAvailable checks whether a knowledge-graph.json exists under
// the workspace root. Kept for backward compatibility.
func understandGraphAvailable(workspaceRoot string) bool {
	if workspaceRoot == "" {
		return false
	}
	_, err := os.Stat(filepath.Join(workspaceRoot, ".understand-anything", "knowledge-graph.json"))
	return err == nil
}

// buildGraphStatus reads the knowledge graph and meta.json to produce a
// status summary for the frontend. Returns nil when no graph exists or
// reading fails — callers treat nil as "no indicator to show".
func buildGraphStatus(workspaceRoot string) *UnderstandGraphStatusView {
	if workspaceRoot == "" {
		return nil
	}
	graphPath := filepath.Join(workspaceRoot, ".understand-anything", "knowledge-graph.json")
	graphData, err := os.ReadFile(graphPath)
	if err != nil {
		return nil
	}

	// Count nodes and edges from the JSON without full unmarshal overhead.
	// We only need the lengths, so a simple bracket-count scan is enough.
	nodeCount := 0
	edgeCount := 0
	// Count "nodes": [...] and "edges": [...] entries
	nodeCount = countJSONArray(graphData, `"nodes"`)
	edgeCount = countJSONArray(graphData, `"edges"`)

	status := &UnderstandGraphStatusView{
		Available: true,
		NodeCount: nodeCount,
		EdgeCount: edgeCount,
	}

	// Check staleness via meta.json
	metaPath := filepath.Join(workspaceRoot, ".understand-anything", "meta.json")
	metaData, err := os.ReadFile(metaPath)
	if err != nil {
		return status // no meta = can't determine staleness, assume not stale
	}

	var meta struct {
		GitCommitHash string `json:"gitCommitHash"`
	}
	if err := json.Unmarshal(metaData, &meta); err != nil || meta.GitCommitHash == "" {
		return status
	}

	current := currentGitHashStatus(workspaceRoot)
	if current == "" || current == meta.GitCommitHash {
		return status
	}
	status.Stale = true
	return status
}

// countJSONArray counts the number of elements in a top-level JSON array key.
// Uses a lightweight approach: find the key, then count top-level commas and
// at least one element.
func countJSONArray(data []byte, key string) int {
	idx := indexOf(data, key)
	if idx < 0 {
		return 0
	}
	// Find opening '[' after the key
	start := idx + len(key)
	bracket := indexOfByte(data[start:], '[')
	if bracket < 0 {
		return 0
	}
	start += bracket + 1
	// Find matching ']' (simple scan, not nested-array safe but good enough
	// for knowledge-graph.json which has flat top-level arrays)
	depth := 1
	end := start
	for end < len(data) && depth > 0 {
		switch data[end] {
		case '[':
			depth++
		case ']':
			depth--
		}
		end++
	}
	if depth != 0 {
		return 0
	}
	arr := data[start : end-1]
	arr = trimSpace(arr)
	if len(arr) == 0 {
		return 0
	}
	// Count top-level commas + 1
	count := 1
	inString := false
	escaped := false
	innerDepth := 0
	for _, b := range arr {
		if escaped {
			escaped = false
			continue
		}
		if b == '\\' && inString {
			escaped = true
			continue
		}
		if b == '"' {
			inString = !inString
			continue
		}
		if inString {
			continue
		}
		if b == '{' || b == '[' {
			innerDepth++
		}
		if b == '}' || b == ']' {
			innerDepth--
		}
		if b == ',' && innerDepth == 0 {
			count++
		}
	}
	return count
}

// currentGitHashStatus runs git rev-parse with a 1-second timeout to avoid
// blocking the Settings() call if git hangs.
func currentGitHashStatus(workDir string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "-C", workDir, "rev-parse", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func indexOf(data []byte, sub string) int {
	limit := len(data) - len(sub)
	for i := 0; i <= limit; i++ {
		if string(data[i:i+len(sub)]) == sub {
			return i
		}
	}
	return -1
}

func indexOfByte(data []byte, b byte) int {
	for i, c := range data {
		if c == b {
			return i
		}
	}
	return -1
}

func trimSpace(data []byte) []byte {
	start := 0
	for start < len(data) && (data[start] == ' ' || data[start] == '\t' || data[start] == '\n' || data[start] == '\r') {
		start++
	}
	end := len(data) - 1
	for end >= start && (data[end] == ' ' || data[end] == '\t' || data[end] == '\n' || data[end] == '\r') {
		end--
	}
	return data[start : end+1]
}
