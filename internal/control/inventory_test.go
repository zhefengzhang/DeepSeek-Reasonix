package control

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"reasonix/internal/provider"
)

func msg(role provider.Role, content string) provider.Message {
	return provider.Message{Role: role, Content: content}
}

func assistantMsg(toolCalls ...provider.ToolCall) provider.Message {
	return provider.Message{Role: provider.RoleAssistant, ToolCalls: toolCalls}
}

func toolMsg(toolCallID, name, content string) provider.Message {
	return provider.Message{Role: provider.RoleTool, ToolCallID: toolCallID, Name: name, Content: content}
}

func tc(id, name, args string) provider.ToolCall {
	return provider.ToolCall{ID: id, Name: name, Arguments: args}
}

// --- Tests ---

func TestReadFileTrackerEmptyHistory(t *testing.T) {
	var tr readFileTracker
	tr.scanDelta(nil)
	if out := tr.build(); out != "" {
		t.Fatalf("empty history: expected \"\", got %q", out)
	}
}

func TestReadFileTrackerSingleRead(t *testing.T) {
	var tr readFileTracker
	msgs := []provider.Message{
		assistantMsg(tc("c1", "read_file", `{"path":"ch1.md","offset":0,"limit":2000}`)),
		toolMsg("c1", "read_file", "   1→line one\n   2→line two"),
	}
	tr.scanDelta(msgs)

	out := tr.build()
	if !strings.Contains(out, "ch1.md") {
		t.Fatalf("expected ch1.md in inventory, got %q", out)
	}
	if !strings.Contains(out, "L1-L2000") {
		t.Fatalf("expected L1-L2000 range, got %q", out)
	}
	if strings.Contains(out, "⚠️ stale") {
		t.Fatalf("file should not be marked stale, got %q", out)
	}
}

func TestReadFileTrackerReadThenEdit(t *testing.T) {
	var tr readFileTracker
	msgs := []provider.Message{
		assistantMsg(tc("c1", "read_file", `{"path":"ch1.md","offset":0,"limit":2000}`)),
		toolMsg("c1", "read_file", "   1→line one"),
		assistantMsg(tc("c2", "edit_file", `{"path":"ch1.md","old_string":"x","new_string":"y"}`)),
		toolMsg("c2", "edit_file", "edit applied"),
	}
	tr.scanDelta(msgs)

	out := tr.build()
	if !strings.Contains(out, "⚠️ stale") {
		t.Fatalf("file should be marked stale after edit, got %q", out)
	}
}

func TestReadFileTrackerReadThenMultiEdit(t *testing.T) {
	var tr readFileTracker
	msgs := []provider.Message{
		assistantMsg(tc("c1", "read_file", `{"path":"ch1.md","offset":0,"limit":2000}`)),
		toolMsg("c1", "read_file", "   1→line one"),
		assistantMsg(tc("c2", "multi_edit", `{"path":"ch1.md","edits":[{"old_string":"x","new_string":"y"}]}`)),
		toolMsg("c2", "multi_edit", "multi_edit applied"),
	}
	tr.scanDelta(msgs)

	out := tr.build()
	if !strings.Contains(out, "⚠️ stale") {
		t.Fatalf("file should be marked stale after multi_edit, got %q", out)
	}
}

func TestReadFileTrackerReadThenWrite(t *testing.T) {
	var tr readFileTracker
	msgs := []provider.Message{
		assistantMsg(tc("c1", "read_file", `{"path":"ch1.md","offset":0,"limit":2000}`)),
		toolMsg("c1", "read_file", "   1→line one"),
		assistantMsg(tc("c2", "write_file", `{"path":"ch1.md","content":"new content"}`)),
		toolMsg("c2", "write_file", "file written"),
	}
	tr.scanDelta(msgs)

	out := tr.build()
	if !strings.Contains(out, "⚠️ stale") {
		t.Fatalf("file should be marked stale after write_file, got %q", out)
	}
}

func TestReadFileTrackerEditThenReReadResetsStale(t *testing.T) {
	var tr readFileTracker
	msgs := []provider.Message{
		// First read
		assistantMsg(tc("c1", "read_file", `{"path":"ch1.md","offset":0,"limit":2000}`)),
		toolMsg("c1", "read_file", "   1→line one"),
		// Edit
		assistantMsg(tc("c2", "edit_file", `{"path":"ch1.md","old_string":"x","new_string":"y"}`)),
		toolMsg("c2", "edit_file", "edit applied"),
		// Re-read resets stale
		assistantMsg(tc("c3", "read_file", `{"path":"ch1.md","offset":0,"limit":2000}`)),
		toolMsg("c3", "read_file", "   1→line one\n   2→line two"),
	}
	tr.scanDelta(msgs)

	out := tr.build()
	if strings.Contains(out, "⚠️ stale") {
		t.Fatalf("file should NOT be stale after re-read, got %q", out)
	}
}

func TestReadFileTrackerMoveFileMarksBothStale(t *testing.T) {
	var tr readFileTracker
	msgs := []provider.Message{
		// Read two files
		assistantMsg(tc("c1", "read_file", `{"path":"a.md","offset":0,"limit":100}`)),
		toolMsg("c1", "read_file", "   1→a"),
		assistantMsg(tc("c2", "read_file", `{"path":"b.md","offset":0,"limit":100}`)),
		toolMsg("c2", "read_file", "   1→b"),
		// Move a → b
		assistantMsg(tc("c3", "move_file", `{"source_path":"a.md","destination_path":"b.md"}`)),
		toolMsg("c3", "move_file", "moved"),
	}
	tr.scanDelta(msgs)

	out := tr.build()
	if !strings.Contains(out, "a.md") || !strings.Contains(out, "b.md") {
		t.Fatalf("expected both paths in inventory, got %q", out)
	}
	// Both should be stale
	if strings.Count(out, "⚠️ stale") != 2 {
		t.Fatalf("expected both files stale after move, got %q", out)
	}
}

func TestReadFileTrackerIncrementalScan(t *testing.T) {
	var tr readFileTracker

	// Scan first batch
	msgs1 := []provider.Message{
		assistantMsg(tc("c1", "read_file", `{"path":"a.md","offset":0,"limit":100}`)),
		toolMsg("c1", "read_file", "   1→a"),
	}
	tr.scanDelta(msgs1)

	// Scan second batch (incremental)
	msgs2 := []provider.Message{
		assistantMsg(tc("c1", "read_file", `{"path":"a.md","offset":0,"limit":100}`)),
		toolMsg("c1", "read_file", "   1→a"),
		assistantMsg(tc("c2", "read_file", `{"path":"b.md","offset":0,"limit":50}`)),
		toolMsg("c2", "read_file", "   1→b"),
	}
	tr.scanDelta(msgs2)

	out := tr.build()
	if !strings.Contains(out, "a.md") {
		t.Fatalf("expected a.md in inventory, got %q", out)
	}
	if !strings.Contains(out, "b.md") {
		t.Fatalf("expected b.md in inventory, got %q", out)
	}
}

func TestReadFileTrackerCompactRebuilds(t *testing.T) {
	var tr readFileTracker

	// First full scan
	msgs1 := []provider.Message{
		assistantMsg(tc("c1", "read_file", `{"path":"a.md","offset":0,"limit":100}`)),
		toolMsg("c1", "read_file", "   1→a"),
		assistantMsg(tc("c2", "read_file", `{"path":"b.md","offset":0,"limit":50}`)),
		toolMsg("c2", "read_file", "   1→b"),
	}
	tr.scanDelta(msgs1)

	// Compact: history shrinks (b.md is gone)
	msgs2 := []provider.Message{
		assistantMsg(tc("c1", "read_file", `{"path":"a.md","offset":0,"limit":100}`)),
		toolMsg("c1", "read_file", "   1→a"),
	}
	tr.scanDelta(msgs2)

	out := tr.build()
	if !strings.Contains(out, "a.md") {
		t.Fatalf("expected a.md in inventory, got %q", out)
	}
	if strings.Contains(out, "b.md") {
		t.Fatalf("b.md should be gone after compact, got %q", out)
	}
}

func TestReadFileTrackerOnlyReadsListed(t *testing.T) {
	var tr readFileTracker
	msgs := []provider.Message{
		// write_file without preceding read → not tracked
		assistantMsg(tc("c1", "write_file", `{"path":"new.md","content":"x"}`)),
		toolMsg("c1", "write_file", "ok"),
		// read_file should show up
		assistantMsg(tc("c2", "read_file", `{"path":"read.md","offset":10,"limit":500}`)),
		toolMsg("c2", "read_file", "  11→line eleven"),
	}
	tr.scanDelta(msgs)

	out := tr.build()
	if strings.Contains(out, "new.md") {
		t.Fatalf("write-only file should not be in inventory, got %q", out)
	}
	if !strings.Contains(out, "read.md") {
		t.Fatalf("read file should be in inventory, got %q", out)
	}
	if !strings.Contains(out, "L11-L510") {
		t.Fatalf("expected L11-L510 range (offset 10, limit 500), got %q", out)
	}
}

func TestReadFileTrackerMultipleReadSameFile(t *testing.T) {
	var tr readFileTracker
	msgs := []provider.Message{
		// First read: offset 0, limit 500
		assistantMsg(tc("c1", "read_file", `{"path":"ch.md","offset":0,"limit":500}`)),
		toolMsg("c1", "read_file", "   1→..."),
		// Second read: offset 400, limit 300 — but last read wins
		assistantMsg(tc("c2", "read_file", `{"path":"ch.md","offset":400,"limit":300}`)),
		toolMsg("c2", "read_file", " 401→..."),
	}
	tr.scanDelta(msgs)

	out := tr.build()
	// Last read wins: offset 400, limit 300, so L401-L700
	if !strings.Contains(out, "L401-L700") {
		t.Fatalf("expected last read range L401-L700, got %q", out)
	}
}

func TestReadFileTrackerLookup(t *testing.T) {
	var tr readFileTracker

	// Lookup on empty tracker
	if _, ok := tr.lookup("x.md"); ok {
		t.Fatal("lookup on empty tracker should return false")
	}

	// Read a file — scanDelta takes the FULL cumulative history each time
	msgs := []provider.Message{
		assistantMsg(tc("c1", "read_file", `{"path":"ch.md","offset":0,"limit":500}`)),
		toolMsg("c1", "read_file", "   1→..."),
	}
	tr.scanDelta(msgs)

	// Lookup should find it
	s, ok := tr.lookup("ch.md")
	if !ok {
		t.Fatal("lookup should find tracked file")
	}
	if s.readOffset != 0 || s.readLimit != 500 {
		t.Fatalf("expected offset=0 limit=500, got offset=%d limit=%d", s.readOffset, s.readLimit)
	}

	// Edit the file — pass full cumulative history
	msgs = append(msgs,
		assistantMsg(tc("c2", "edit_file", `{"path":"ch.md","old_string":"x","new_string":"y"}`)),
		toolMsg("c2", "edit_file", "applied"),
	)
	tr.scanDelta(msgs)

	// Lookup should NOT find it after edit (stale)
	if _, ok := tr.lookup("ch.md"); ok {
		t.Fatal("lookup should not find stale file")
	}

	// Re-read resets stale — pass full cumulative history
	msgs = append(msgs,
		assistantMsg(tc("c3", "read_file", `{"path":"ch.md","offset":0,"limit":500}`)),
		toolMsg("c3", "read_file", "   1→..."),
	)
	tr.scanDelta(msgs)

	if _, ok := tr.lookup("ch.md"); !ok {
		t.Fatal("lookup should find file after re-read")
	}
}

func TestReadFileTrackerSameTurnReRead(t *testing.T) {
	var tr readFileTracker

	// Simulate scanDelta at Compose time (clears inTurn, processes history)
	tr.scanDelta(nil)

	// First read this turn: trackInTurn records it
	tr.trackInTurn("a.md", 0, 500)
	if _, ok := tr.lookup("a.md"); !ok {
		t.Fatal("lookup should find file tracked inTurn")
	}

	// Same turn, same path: lookup should still find it (via inTurn)
	if _, ok := tr.lookup("a.md"); !ok {
		t.Fatal("lookup should find file in inTurn on second call")
	}

	// Different path, not tracked → not found
	if _, ok := tr.lookup("b.md"); ok {
		t.Fatal("lookup should not find untracked file")
	}

	// Next turn: scanDelta clears inTurn
	tr.scanDelta(nil)
	if _, ok := tr.lookup("a.md"); ok {
		t.Fatal("lookup should not find file after scanDelta clears inTurn")
	}
}

// TestPreCheckClosureWithTracker simulates the exact closure used in
// newInteractiveGate() — the PreCheck calls readFileTracker.lookup and
// readFileTracker.trackInTurn. This is the bridge between the unit-tested
// tracker and the Agent-level gate test.
func TestPreCheckClosureWithTracker(t *testing.T) {
	var tr readFileTracker

	// Build the exact same closure used in newInteractiveGate().
	preCheck := func(toolName string, args json.RawMessage) (bool, string) {
		if toolName != "read_file" {
			return false, ""
		}
		var p struct {
			Path   string `json:"path"`
			Offset int    `json:"offset"`
			Limit  int    `json:"limit"`
		}
		if err := json.Unmarshal(args, &p); err != nil || p.Path == "" {
			return false, ""
		}
		s, ok := tr.lookup(p.Path)
		if ok {
			subject := "re-read " + p.Path + " — already in context"
			if s.readLimit > 0 {
				subject += " (L" + strconv.Itoa(s.readOffset+1) + "-L" + strconv.Itoa(s.readOffset+s.readLimit) + ")"
			}
			return true, subject
		}
		tr.trackInTurn(p.Path, p.Offset, p.Limit)
		return false, ""
	}

	// First call: should return false (records the read).
	ask, _ := preCheck("read_file", json.RawMessage(`{"path":"a.md","offset":0,"limit":500}`))
	if ask {
		t.Fatal("first call should return false (not a re-read)")
	}

	// Second call, same path: should return true (re-read detected).
	ask, subject := preCheck("read_file", json.RawMessage(`{"path":"a.md","offset":0,"limit":500}`))
	if !ask {
		t.Fatal("second call should return true (re-read detected)")
	}
	if !strings.Contains(subject, "re-read") {
		t.Fatalf("subject should contain 're-read', got %q", subject)
	}
	if !strings.Contains(subject, "L1-L500") {
		t.Fatalf("subject should contain range, got %q", subject)
	}

	// Different path: should NOT trigger.
	ask, _ = preCheck("read_file", json.RawMessage(`{"path":"b.md","offset":100,"limit":300}`))
	if ask {
		t.Fatal("different path should not trigger re-read")
	}

	// Non-read_file tool: should return false.
	ask, _ = preCheck("edit_file", json.RawMessage(`{"path":"a.md"}`))
	if ask {
		t.Fatal("non-read_file should return false")
	}
}
