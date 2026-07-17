package control

import (
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
