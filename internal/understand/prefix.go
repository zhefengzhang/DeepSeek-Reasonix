// Package understand provides shared utilities for the Understand-Anything
// knowledge graph, including cache-stable system-prompt prefix injection.
package understand

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// --- graph types (subset of the UA schema) ---

type uaProjectMeta struct {
	Name        string   `json:"name"`
	Languages   []string `json:"languages"`
	Frameworks  []string `json:"frameworks"`
	Description string   `json:"description"`
	GitCommit   string   `json:"gitCommitHash"`
}

type uaNode struct {
	ID         string   `json:"id"`
	Type       string   `json:"type"`
	Name       string   `json:"name"`
	FilePath   string   `json:"filePath,omitempty"`
	Summary    string   `json:"summary"`
	Tags       []string `json:"tags"`
	Complexity string   `json:"complexity"`
}

type uaEdge struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Type   string `json:"type"`
}

type uaLayer struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	NodeIDs     []string `json:"nodeIds"`
}

type uaGraph struct {
	Version string        `json:"version"`
	Project uaProjectMeta `json:"project"`
	Nodes   []uaNode      `json:"nodes"`
	Edges   []uaEdge      `json:"edges"`
	Layers  []uaLayer     `json:"layers"`
}

// Prefix reads the knowledge graph from workspaceRoot and returns a concise,
// cache-stable text summary suitable for injection into the system prompt.
// Returns "" when no graph exists or it can't be read — callers simply skip
// injection. The output is deterministic: identical graph → identical bytes,
// so DeepSeek's automatic prefix cache stays warm across turns.
func Prefix(workspaceRoot string) string {
	if workspaceRoot == "" {
		return ""
	}
	path := filepath.Join(workspaceRoot, ".understand-anything", "knowledge-graph.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var g uaGraph
	if err := json.Unmarshal(data, &g); err != nil {
		return ""
	}
	return buildPrefix(&g)
}

// TurnHint returns a compact one-liner for per-turn injection (turn tail).
// It tells the model which code intelligence tools are available and suggests
// using understand_search + codegraph_explore together. Returns "" when no
// graph exists. Unlike Prefix(), this is NOT cache-stable — it's designed to
// be injected every turn via control.Compose(), so staleness is checked fresh
// each time.
func TurnHint(workspaceRoot string) string {
	if workspaceRoot == "" {
		return ""
	}
	path := filepath.Join(workspaceRoot, ".understand-anything", "knowledge-graph.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var g uaGraph
	if err := json.Unmarshal(data, &g); err != nil {
		return ""
	}

	staleNote := ""
	metaPath := filepath.Join(workspaceRoot, ".understand-anything", "meta.json")
	if metaData, err := os.ReadFile(metaPath); err == nil {
		var m struct {
			GitCommitHash string `json:"gitCommitHash"`
		}
		if json.Unmarshal(metaData, &m) == nil && m.GitCommitHash != "" {
			if cur := currentGitHash(workspaceRoot); cur != "" && cur != m.GitCommitHash {
				staleNote = " ⚠️ STALE — run /understand to refresh."
			}
		}
	}

	// Detect CodeGraph availability so the hint includes both tools.
	codeGraphHint := ""
	if _, err := os.Stat(filepath.Join(workspaceRoot, ".codegraph")); err == nil {
		codeGraphHint = " + `codegraph_explore` (`mcp__codegraph__codegraph_explore`) for source-level queries (verbatim source, call paths, blast radius)"
	}

	return fmt.Sprintf("💡 Knowledge graph: %d nodes, %d edges.%s Use `understand_search` for structural exploration (modules, files, layers, relationships)%s. Prefer these over grep for codebase exploration.", len(g.Nodes), len(g.Edges), staleNote, codeGraphHint)
}

// currentGitHash runs git rev-parse with a 1s timeout.
func currentGitHash(workDir string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "-C", workDir, "rev-parse", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

const (
	prefixMaxChars    = 2000
	prefixMaxLayers   = 10
	prefixMaxEntryPts = 5
)

func buildPrefix(g *uaGraph) string {
	var b strings.Builder

	// Project header
	b.WriteString("# Project Overview\n")
	if g.Project.Name != "" {
		fmt.Fprintf(&b, "**%s**", g.Project.Name)
	}
	if g.Project.Description != "" {
		if g.Project.Name != "" {
			b.WriteString(" — ")
		}
		b.WriteString(truncateStr(g.Project.Description, 200))
	}
	b.WriteString("\n")

	if len(g.Project.Languages) > 0 || len(g.Project.Frameworks) > 0 {
		b.WriteString("**Languages:** ")
		b.WriteString(strings.Join(g.Project.Languages, ", "))
		if len(g.Project.Frameworks) > 0 {
			b.WriteString(" | **Frameworks:** ")
			b.WriteString(strings.Join(g.Project.Frameworks, ", "))
		}
		b.WriteString("\n")
	}
	b.WriteString("\n")

	// Layers
	if len(g.Layers) > 0 {
		b.WriteString("## Architecture\n")
		layers := g.Layers
		if len(layers) > prefixMaxLayers {
			layers = layers[:prefixMaxLayers]
		}
		for _, l := range layers {
			fmt.Fprintf(&b, "- **%s**: %s\n", l.Name, truncateStr(l.Description, 120))
		}
		b.WriteString("\n")
	}

	// Key entry points (modules and concepts)
	entries := make([]string, 0)
	for _, n := range g.Nodes {
		if n.Type == "module" || n.Type == "concept" {
			entries = append(entries, n.Name)
		}
	}
	if len(entries) > 0 {
		sort.Strings(entries)
		if len(entries) > prefixMaxEntryPts {
			entries = entries[:prefixMaxEntryPts]
		}
		b.WriteString("**Key modules:** ")
		b.WriteString(strings.Join(entries, ", "))
		b.WriteString("\n")
	}

	// Stats
	nodeTypes := make(map[string]int)
	for _, n := range g.Nodes {
		nodeTypes[n.Type]++
	}
	typeCounts := make([]string, 0, len(nodeTypes))
	for t, c := range nodeTypes {
		typeCounts = append(typeCounts, fmt.Sprintf("%d %s", c, t))
	}
	sort.Strings(typeCounts)
	fmt.Fprintf(&b, "**%d nodes** (%s) | **%d edges**", len(g.Nodes), strings.Join(typeCounts, ", "), len(g.Edges))

	// Commit hash (stable identifier)
	if g.Project.GitCommit != "" {
		short := g.Project.GitCommit
		if len(short) > 8 {
			short = short[:8]
		}
		fmt.Fprintf(&b, " | **commit:** `%s`", short)
	}
	b.WriteString("\n")

	// Usage hint — nudge the agent toward understand_search for structural queries.
	b.WriteString("\n> 💡 Use `understand_search` for structural exploration — it's faster than grep for finding modules, layers, and relationships.\n")

	result := b.String()
	if len(result) > prefixMaxChars {
		result = result[:prefixMaxChars-3] + "..."
	}
	return result
}

func truncateStr(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.Join(strings.Fields(s), " ")
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
