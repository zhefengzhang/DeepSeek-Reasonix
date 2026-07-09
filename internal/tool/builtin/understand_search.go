package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"reasonix/internal/tool"
)

func init() { tool.RegisterBuiltin(understandSearch{}) }

type understandSearch struct {
	workDir string
}

func (understandSearch) Name() string { return "understand_search" }

func (understandSearch) Description() string {
	return "Search the Understand-Anything knowledge graph for structural information about the codebase. Query nodes by name, tags, or summary; get a project outline; inspect a specific node; or traverse edges from a node to discover relationships."
}

func (understandSearch) Schema() json.RawMessage {
	return json.RawMessage(`{
"type":"object",
"properties":{
  "action":{"type":"string","enum":["search","outline","node","traverse","dashboard"],"description":"search: find nodes matching a query; outline: return project structure overview; node: inspect a specific node by ID; traverse: follow edges from a node; dashboard: start the interactive dashboard"},
  "query":{"type":"string","description":"Search query for action=search. Matches against name, tags, and summary."},
  "node_id":{"type":"string","description":"Node ID for action=node or action=traverse (as starting node)."},
  "from":{"type":"string","description":"Starting node ID for action=traverse (alias for node_id)."},
  "direction":{"type":"string","enum":["forward","backward","both"],"description":"Edge direction for action=traverse. Default: forward."},
  "edge_type":{"type":"string","description":"Optional edge type filter for action=traverse (e.g. 'calls', 'imports')."},
  "limit":{"type":"integer","description":"Maximum results for action=search or action=traverse (default 20, max 50).","minimum":1},
  "types":{"type":"array","items":{"type":"string"},"description":"Optional filter: only return nodes of these types (e.g. [\"file\",\"function\",\"class\"])"}
},
"required":["action"]
}`)
}

func (understandSearch) ReadOnly() bool { return true }

const (
	understandDefaultLimit = 20
	understandMaxLimit     = 50
)

type understandArgs struct {
	Action    string   `json:"action"`
	Query     string   `json:"query"`
	NodeID    string   `json:"node_id"`
	From      string   `json:"from"`
	Direction string   `json:"direction"`
	EdgeType  string   `json:"edge_type"`
	Limit     int      `json:"limit"`
	Types     []string `json:"types"`
}

func (u understandSearch) graphPath() string {
	return resolveIn(u.workDir, filepath.Join(".understand-anything", "knowledge-graph.json"))
}

func (u understandSearch) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	p := understandArgs{Limit: understandDefaultLimit}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}
	p.Action = strings.ToLower(strings.TrimSpace(p.Action))

	switch p.Action {
	case "search":
		return u.doSearch(p)
	case "outline":
		return u.doOutline(p)
	case "node":
		return u.doNode(p)
	case "traverse":
		return u.doTraverse(p)
	case "dashboard":
		return u.doDashboard(p)
	default:
		return "", fmt.Errorf("action must be search, outline, node, traverse, or dashboard")
	}
}

// --- graph types ---

type uaProjectMeta struct {
	Name        string   `json:"name"`
	Languages   []string `json:"languages"`
	Frameworks  []string `json:"frameworks"`
	Description string   `json:"description"`
	AnalyzedAt  string   `json:"analyzedAt"`
}

type uaNode struct {
	ID            string   `json:"id"`
	Type          string   `json:"type"`
	Name          string   `json:"name"`
	FilePath      string   `json:"filePath,omitempty"`
	LineRange     []int    `json:"lineRange,omitempty"`
	Summary       string   `json:"summary"`
	Tags          []string `json:"tags"`
	Complexity    string   `json:"complexity"`
	LanguageNotes string   `json:"languageNotes,omitempty"`
}

type uaEdge struct {
	Source      string  `json:"source"`
	Target      string  `json:"target"`
	Type        string  `json:"type"`
	Direction   string  `json:"direction"`
	Description string  `json:"description,omitempty"`
	Weight      float64 `json:"weight"`
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

func (u understandSearch) loadGraph() (*uaGraph, error) {
	path := u.graphPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("no knowledge graph found at %s — run /understand first to generate it", path)
		}
		return nil, fmt.Errorf("failed to read knowledge graph: %w", err)
	}
	var g uaGraph
	if err := json.Unmarshal(data, &g); err != nil {
		return nil, fmt.Errorf("failed to parse knowledge graph at %s: %w", path, err)
	}
	return &g, nil
}

type uaMeta struct {
	LastAnalyzedAt string `json:"lastAnalyzedAt"`
	GitCommitHash  string `json:"gitCommitHash"`
	Version        string `json:"version"`
	AnalyzedFiles  int    `json:"analyzedFiles"`
}

func (u understandSearch) metaPath() string {
	return resolveIn(u.workDir, filepath.Join(".understand-anything", "meta.json"))
}

func (u understandSearch) currentGitHash() string {
	workDir := u.workDir
	if workDir == "" {
		var err error
		workDir, err = os.Getwd()
		if err != nil {
			return ""
		}
	}
	cmd := exec.Command("git", "-C", workDir, "rev-parse", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func (u understandSearch) stalenessNote() string {
	metaPath := u.metaPath()
	data, err := os.ReadFile(metaPath)
	if err != nil {
		return ""
	}
	var m uaMeta
	if err := json.Unmarshal(data, &m); err != nil {
		return ""
	}
	if m.GitCommitHash == "" {
		return ""
	}
	current := u.currentGitHash()
	if current == "" {
		return ""
	}
	if m.GitCommitHash != current {
		shortMeta := m.GitCommitHash
		if len(shortMeta) > 8 {
			shortMeta = shortMeta[:8]
		}
		shortCur := current
		if len(shortCur) > 8 {
			shortCur = shortCur[:8]
		}
		return fmt.Sprintf("\n> ⚠️ **Graph may be stale.** Analyzed at commit `%s` but HEAD is `%s`. Run `/understand` to refresh.\n", shortMeta, shortCur)
	}
	return ""
}

// --- search ---

type searchResult struct {
	Node  uaNode  `json:"node"`
	Score float64 `json:"score"`
}

func (u understandSearch) doSearch(p understandArgs) (string, error) {
	if strings.TrimSpace(p.Query) == "" {
		return "", fmt.Errorf("query is required for search action")
	}
	if p.Limit <= 0 {
		p.Limit = understandDefaultLimit
	}
	if p.Limit > understandMaxLimit {
		p.Limit = understandMaxLimit
	}

	g, err := u.loadGraph()
	if err != nil {
		return "", err
	}

	typeFilter := make(map[string]bool)
	for _, t := range p.Types {
		typeFilter[strings.TrimSpace(t)] = true
	}

	query := strings.ToLower(strings.TrimSpace(p.Query))
	tokens := strings.Fields(query)

	var results []searchResult
	for _, n := range g.Nodes {
		if len(typeFilter) > 0 && !typeFilter[n.Type] {
			continue
		}
		score := matchScore(n, tokens)
		if score > 0 {
			results = append(results, searchResult{Node: n, Score: score})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if len(results) > p.Limit {
		results = results[:p.Limit]
	}

	out := &strings.Builder{}
	if note := u.stalenessNote(); note != "" {
		out.WriteString(note)
	}
	fmt.Fprintf(out, "Found %d matching nodes", len(results))
	if len(results) == 0 {
		fmt.Fprintf(out, " — try broadening your query.\n")
		return out.String(), nil
	}
	fmt.Fprintf(out, ":\n\n")
	for i, r := range results {
		fmt.Fprintf(out, "%d. **%s** (%s)", i+1, r.Node.Name, r.Node.Type)
		if r.Node.FilePath != "" {
			fmt.Fprintf(out, " — `%s`", r.Node.FilePath)
		}
		fmt.Fprintf(out, "\n   ID: `%s` | Score: %.2f\n", r.Node.ID, r.Score)
		if r.Node.Summary != "" {
			fmt.Fprintf(out, "   %s\n", r.Node.Summary)
		}
		if len(r.Node.Tags) > 0 {
			fmt.Fprintf(out, "   Tags: %s\n", strings.Join(r.Node.Tags, ", "))
		}
		fmt.Fprintf(out, "\n")
	}
	return out.String(), nil
}

func matchScore(n uaNode, tokens []string) float64 {
	if len(tokens) == 0 {
		return 0
	}
	var score float64
	nameLower := strings.ToLower(n.Name)
	summaryLower := strings.ToLower(n.Summary)
	notesLower := strings.ToLower(n.LanguageNotes)

	for _, tok := range tokens {
		tokWeight := 0.0
		if strings.Contains(nameLower, tok) {
			tokWeight += 0.4
		}
		for _, tag := range n.Tags {
			if strings.Contains(strings.ToLower(tag), tok) {
				tokWeight += 0.3
				break
			}
		}
		if strings.Contains(summaryLower, tok) {
			tokWeight += 0.2
		}
		if strings.Contains(notesLower, tok) {
			tokWeight += 0.1
		}
		score += tokWeight
	}
	return score / float64(len(tokens))
}

// --- outline ---

func (u understandSearch) doOutline(p understandArgs) (string, error) {
	g, err := u.loadGraph()
	if err != nil {
		return "", err
	}

	out := &strings.Builder{}
	if note := u.stalenessNote(); note != "" {
		out.WriteString(note)
	}
	fmt.Fprintf(out, "# %s\n\n", g.Project.Name)
	if g.Project.Description != "" {
		fmt.Fprintf(out, "%s\n\n", g.Project.Description)
	}
	fmt.Fprintf(out, "**Languages:** %s\n", strings.Join(g.Project.Languages, ", "))
	if len(g.Project.Frameworks) > 0 {
		fmt.Fprintf(out, "**Frameworks:** %s\n", strings.Join(g.Project.Frameworks, ", "))
	}
	fmt.Fprintf(out, "**Analyzed:** %s | **Nodes:** %d | **Edges:** %d\n\n", g.Project.AnalyzedAt, len(g.Nodes), len(g.Edges))

	if len(g.Layers) > 0 {
		fmt.Fprintf(out, "## Layers\n\n")
		for _, l := range g.Layers {
			fmt.Fprintf(out, "- **%s** (%d nodes): %s\n", l.Name, len(l.NodeIDs), l.Description)
		}
		fmt.Fprintf(out, "\n")
	}

	fileNodes := make([]uaNode, 0)
	moduleNodes := make([]uaNode, 0)
	conceptNodes := make([]uaNode, 0)
	for _, n := range g.Nodes {
		switch n.Type {
		case "file":
			fileNodes = append(fileNodes, n)
		case "module":
			moduleNodes = append(moduleNodes, n)
		case "concept":
			conceptNodes = append(conceptNodes, n)
		}
	}

	if len(moduleNodes) > 0 {
		fmt.Fprintf(out, "## Modules (%d)\n\n", len(moduleNodes))
		for _, m := range moduleNodes {
			fmt.Fprintf(out, "- **%s**: %s\n", m.Name, m.Summary)
		}
		fmt.Fprintf(out, "\n")
	}

	if len(conceptNodes) > 0 {
		fmt.Fprintf(out, "## Concepts (%d)\n\n", len(conceptNodes))
		for _, c := range conceptNodes {
			fmt.Fprintf(out, "- **%s**: %s\n", c.Name, c.Summary)
		}
		fmt.Fprintf(out, "\n")
	}

	fmt.Fprintf(out, "## Files (%d)\n\n", len(fileNodes))
	sort.Slice(fileNodes, func(i, j int) bool { return fileNodes[i].FilePath < fileNodes[j].FilePath })
	for _, f := range fileNodes {
		fmt.Fprintf(out, "- `%s` — %s [%s]\n", f.FilePath, truncate(f.Summary, 100), f.Complexity)
	}

	if len(g.Nodes) > len(fileNodes)+len(moduleNodes)+len(conceptNodes) {
		fmt.Fprintf(out, "\n*+ %d additional nodes (functions, classes, services, configs, etc.). Use `understand_search({ action: \"search\", query: \"...\" })` to explore.*\n",
			len(g.Nodes)-len(fileNodes)-len(moduleNodes)-len(conceptNodes))
	}

	return out.String(), nil
}

// --- node ---

func (u understandSearch) doNode(p understandArgs) (string, error) {
	nodeID := strings.TrimSpace(p.NodeID)
	if nodeID == "" {
		return "", fmt.Errorf("node_id is required for node action")
	}

	g, err := u.loadGraph()
	if err != nil {
		return "", err
	}

	var target *uaNode
	for i := range g.Nodes {
		if g.Nodes[i].ID == nodeID {
			target = &g.Nodes[i]
			break
		}
	}
	if target == nil {
		return "", fmt.Errorf("node %q not found in knowledge graph", nodeID)
	}

	out := &strings.Builder{}
	if note := u.stalenessNote(); note != "" {
		out.WriteString(note)
	}
	fmt.Fprintf(out, "# %s\n\n", target.Name)
	fmt.Fprintf(out, "- **ID:** `%s`\n", target.ID)
	fmt.Fprintf(out, "- **Type:** %s\n", target.Type)
	if target.FilePath != "" {
		fmt.Fprintf(out, "- **File:** `%s`", target.FilePath)
		if len(target.LineRange) == 2 {
			fmt.Fprintf(out, " (lines %d-%d)", target.LineRange[0], target.LineRange[1])
		}
		fmt.Fprintf(out, "\n")
	}
	fmt.Fprintf(out, "- **Complexity:** %s\n", target.Complexity)
	if len(target.Tags) > 0 {
		fmt.Fprintf(out, "- **Tags:** %s\n", strings.Join(target.Tags, ", "))
	}
	fmt.Fprintf(out, "\n%s\n", target.Summary)
	if target.LanguageNotes != "" {
		fmt.Fprintf(out, "\n**Language notes:** %s\n", target.LanguageNotes)
	}

	outgoing := make([]uaEdge, 0)
	incoming := make([]uaEdge, 0)
	for _, e := range g.Edges {
		if e.Source == nodeID {
			outgoing = append(outgoing, e)
		}
		if e.Target == nodeID {
			incoming = append(incoming, e)
		}
	}

	if len(outgoing) > 0 {
		fmt.Fprintf(out, "\n## Outgoing edges (%d)\n\n", len(outgoing))
		for _, e := range outgoing {
			targetName := resolveNodeName(g, e.Target)
			fmt.Fprintf(out, "- **%s** → `%s`", e.Type, targetName)
			if e.Description != "" {
				fmt.Fprintf(out, " — %s", e.Description)
			}
			fmt.Fprintf(out, " (weight: %.2f)\n", e.Weight)
		}
	}

	if len(incoming) > 0 {
		fmt.Fprintf(out, "\n## Incoming edges (%d)\n\n", len(incoming))
		for _, e := range incoming {
			sourceName := resolveNodeName(g, e.Source)
			fmt.Fprintf(out, "- `%s` **%s** → this", sourceName, e.Type)
			if e.Description != "" {
				fmt.Fprintf(out, " — %s", e.Description)
			}
			fmt.Fprintf(out, " (weight: %.2f)\n", e.Weight)
		}
	}

	return out.String(), nil
}

// --- traverse ---

func (u understandSearch) doTraverse(p understandArgs) (string, error) {
	fromID := strings.TrimSpace(p.From)
	if fromID == "" {
		fromID = strings.TrimSpace(p.NodeID)
	}
	if fromID == "" {
		return "", fmt.Errorf("from (or node_id) is required for traverse action")
	}
	if p.Limit <= 0 {
		p.Limit = understandDefaultLimit
	}
	if p.Limit > understandMaxLimit {
		p.Limit = understandMaxLimit
	}
	dir := strings.ToLower(strings.TrimSpace(p.Direction))
	if dir == "" {
		dir = "forward"
	}

	g, err := u.loadGraph()
	if err != nil {
		return "", err
	}

	var startNode *uaNode
	for i := range g.Nodes {
		if g.Nodes[i].ID == fromID {
			startNode = &g.Nodes[i]
			break
		}
	}
	if startNode == nil {
		return "", fmt.Errorf("starting node %q not found in knowledge graph", fromID)
	}

	type traverseResult struct {
		edge uaEdge
		node uaNode
	}
	var results []traverseResult
	edgeTypeFilter := strings.TrimSpace(p.EdgeType)

	for _, e := range g.Edges {
		var matches bool
		var targetID string
		switch dir {
		case "forward":
			if e.Source == fromID {
				matches = true
				targetID = e.Target
			}
		case "backward":
			if e.Target == fromID {
				matches = true
				targetID = e.Source
			}
		case "both":
			if e.Source == fromID {
				matches = true
				targetID = e.Target
			} else if e.Target == fromID {
				matches = true
				targetID = e.Source
			}
		}
		if !matches {
			continue
		}
		if edgeTypeFilter != "" && e.Type != edgeTypeFilter {
			continue
		}
		var targetNode *uaNode
		for i := range g.Nodes {
			if g.Nodes[i].ID == targetID {
				targetNode = &g.Nodes[i]
				break
			}
		}
		if targetNode == nil {
			continue
		}
		results = append(results, traverseResult{edge: e, node: *targetNode})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].edge.Weight > results[j].edge.Weight
	})

	if len(results) > p.Limit {
		results = results[:p.Limit]
	}

	out := &strings.Builder{}
	if note := u.stalenessNote(); note != "" {
		out.WriteString(note)
	}
	dirLabel := dir
	if edgeTypeFilter != "" {
		dirLabel += ", type: " + edgeTypeFilter
	}
	fmt.Fprintf(out, "## Traverse from `%s` (%s)\n\n", startNode.Name, dirLabel)
	if startNode.Summary != "" {
		fmt.Fprintf(out, "*%s*\n\n", startNode.Summary)
	}
	if len(results) == 0 {
		fmt.Fprintf(out, "No matching edges found.\n")
		return out.String(), nil
	}
	fmt.Fprintf(out, "Found %d edges:\n\n", len(results))
	for i, r := range results {
		fmt.Fprintf(out, "%d. **%s** (%s)", i+1, r.node.Name, r.node.Type)
		if r.node.FilePath != "" {
			fmt.Fprintf(out, " — `%s`", r.node.FilePath)
		}
		fmt.Fprintf(out, "\n   `%s`", r.edge.Type)
		if r.edge.Description != "" {
			fmt.Fprintf(out, " — %s", r.edge.Description)
		}
		fmt.Fprintf(out, " (weight: %.2f)\n\n", r.edge.Weight)
	}
	return out.String(), nil
}

// --- dashboard ---

func (u understandSearch) doDashboard(p understandArgs) (string, error) {
	path := u.graphPath()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return "", fmt.Errorf("no knowledge graph found at %s — run /understand first to generate it", path)
	}
	return fmt.Sprintf(
		"Dashboard is not yet available as a built-in. To view the knowledge graph:\n\n"+
			"1. Open the dashboard from the Understand-Anything plugin:\n"+
			"   ```bash\n"+
			"   cd OtherPackage/Understand-Anything/understand-anything-plugin/packages/dashboard\n"+
			"   GRAPH_DIR=%s pnpm dev\n"+
			"   ```\n"+
			"2. The knowledge graph is ready at `%s`\n",
		u.workDir, path,
	), nil
}

// --- helpers ---

func resolveNodeName(g *uaGraph, id string) string {
	for _, n := range g.Nodes {
		if n.ID == id {
			if n.FilePath != "" && n.Name != n.FilePath {
				return fmt.Sprintf("%s (%s)", n.Name, n.FilePath)
			}
			return n.Name
		}
	}
	return id
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
