// Package control is the transport-agnostic session driver.
package control

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"

	"reasonix/internal/provider"
)

// fileState tracks what we know about a file in the conversation context.
type fileState struct {
	path       string
	readOffset int  // 0-based offset used in last read_file
	readLimit  int  // limit used in last read_file
	stale      bool // true when file was edited after last read
}

// readFileTracker maintains the read-file inventory for context-reuse hints.
// Call scanDelta at the start of each Compose with the current message history;
// it incrementally processes only new messages. Call build to produce the
// <context-inventory> block injected into the turn.
type readFileTracker struct {
	files    map[string]*fileState // path → state (cross-turn, survives compaction)
	scanUpTo int                   // number of messages already scanned

	// inTurn tracks read_file calls that happened during the current turn,
	// before scanDelta has had a chance to process them. Cleared each Compose.
	// This lets same-turn re-reads trigger the PreCheck guard.
	inTurn map[string]*fileState
}

// scanDelta processes msgs[scanUpTo:] and updates the file inventory. When
// len(msgs) < scanUpTo a compact or session reset has rewritten the history
// and the tracker rebuilds from scratch.
func (t *readFileTracker) scanDelta(msgs []provider.Message) {
	// Clear in-turn tracking each Compose — the new turn starts fresh.
	t.inTurn = nil

	if len(msgs) < t.scanUpTo {
		t.files = nil
		t.scanUpTo = 0
	}
	if t.files == nil {
		t.files = make(map[string]*fileState)
	}
	if len(msgs) == t.scanUpTo {
		return
	}
	t.scanMessages(msgs[t.scanUpTo:])
	t.scanUpTo = len(msgs)
}

// scanMessages processes a slice of messages, updating file state for every
// read_file result and write/edit tool call in order.
func (t *readFileTracker) scanMessages(msgs []provider.Message) {
	// Collect tool-call metadata keyed by call ID so we can match results.
	type callInfo struct {
		name string
		args string
	}
	pending := make(map[string]callInfo)
	for _, m := range msgs {
		if m.Role == provider.RoleAssistant {
			for _, tc := range m.ToolCalls {
				if tc.ID != "" {
					pending[tc.ID] = callInfo{name: tc.Name, args: tc.Arguments}
				}
			}
		}
	}

	// Walk messages in order; tool results carry the trailing state.
	for _, m := range msgs {
		if m.Role != provider.RoleTool || m.ToolCallID == "" {
			continue
		}
		info, ok := pending[m.ToolCallID]
		if !ok {
			continue
		}
		t.apply(info.name, info.args)
	}
}

// apply updates the file inventory for one tool execution.
func (t *readFileTracker) apply(toolName, argsJSON string) {
	switch toolName {
	case "read_file":
		path, offset, limit := parseReadArgs(argsJSON)
		if path == "" {
			return
		}
		s := t.files[path]
		if s == nil {
			s = &fileState{path: path}
			t.files[path] = s
		}
		// A successful read resets staleness and records the range.
		s.readOffset = offset
		s.readLimit = limit
		s.stale = false

	case "write_file", "edit_file", "multi_edit", "delete_range", "delete_symbol", "notebook_edit":
		path := parseSinglePath(argsJSON)
		if path == "" {
			return
		}
		if s := t.files[path]; s != nil {
			s.stale = true
		}

	case "move_file":
		src, dst := parseMovePaths(argsJSON)
		if s := t.files[src]; s != nil {
			s.stale = true
		}
		if s := t.files[dst]; s != nil {
			s.stale = true
		}
	}
}

// lookup returns the fileState for a path when it exists and is not stale.
// It checks inTurn first (same-turn reads), then falls back to files (cross-turn).
func (t *readFileTracker) lookup(path string) (fileState, bool) {
	// Check same-turn reads first.
	if s, ok := t.inTurn[path]; ok && s != nil && !s.stale {
		return *s, true
	}
	s, ok := t.files[path]
	if !ok || s == nil || s.stale {
		return fileState{}, false
	}
	return *s, true
}

// trackInTurn records that read_file was just called for path within the
// current turn, so a subsequent read_file for the same path triggers PreCheck.
func (t *readFileTracker) trackInTurn(path string, offset, limit int) {
	if t.inTurn == nil {
		t.inTurn = make(map[string]*fileState)
	}
	t.inTurn[path] = &fileState{path: path, readOffset: offset, readLimit: limit}
}

// build returns a compact <context-inventory> block, or "" when no files have
// been read. The block is self-describing so the model can act on it even
// without a separate policy rule.
func (t *readFileTracker) build() string {
	// Collect non-empty paths, sorted for stable output.
	paths := make([]string, 0, len(t.files))
	for p := range t.files {
		if p == "" {
			continue
		}
		paths = append(paths, p)
	}
	if len(paths) == 0 {
		return ""
	}
	// Sort for deterministic output (cache-friendly within the turn tail).
	sort.Strings(paths)

	var b strings.Builder
	b.WriteString("<context-inventory>\n")
	b.WriteString("Already in context — do NOT re-read unless marked stale:\n")
	for _, p := range paths {
		s := t.files[p]
		b.WriteString("  ")
		b.WriteString(p)
		if s.readLimit > 0 {
			start := s.readOffset + 1 // 1-based for display
			end := s.readOffset + s.readLimit
			b.WriteString(" (L")
			b.WriteString(strconv.Itoa(start))
			b.WriteString("-L")
			b.WriteString(strconv.Itoa(end))
			b.WriteString(")")
		}
		if s.stale {
			b.WriteString(" ⚠️ stale — was edited after read")
		}
		b.WriteString("\n")
	}
	b.WriteString("</context-inventory>")
	return b.String()
}

// --- argument parsers ---

func parseSinglePath(argsJSON string) string {
	if argsJSON == "" {
		return ""
	}
	var p struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &p); err != nil {
		return ""
	}
	return p.Path
}

func parseReadArgs(argsJSON string) (path string, offset, limit int) {
	if argsJSON == "" {
		return "", 0, 0
	}
	var p struct {
		Path   string `json:"path"`
		Offset int    `json:"offset"`
		Limit  int    `json:"limit"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &p); err != nil {
		return "", 0, 0
	}
	if p.Limit <= 0 {
		p.Limit = 2000 // readFileDefaultLimit
	}
	return p.Path, p.Offset, p.Limit
}

func parseMovePaths(argsJSON string) (src, dst string) {
	if argsJSON == "" {
		return "", ""
	}
	var p struct {
		SourcePath      string `json:"source_path"`
		DestinationPath string `json:"destination_path"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &p); err != nil {
		return "", ""
	}
	return p.SourcePath, p.DestinationPath
}
