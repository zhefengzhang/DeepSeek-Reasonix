package agent

import (
	"context"
	"encoding/json"
	"reasonix/internal/event"
	"reasonix/internal/permission"
	"strings"
	"testing"

	"reasonix/internal/provider"
	"reasonix/internal/tool"
)

// stubGate denies any call whose tool name is in deny; everything else allows.
type stubGate struct {
	deny    map[string]bool
	checked []string
}

func (g *stubGate) Check(ctx context.Context, toolName string, args json.RawMessage, readOnly bool) (bool, string, error) {
	g.checked = append(g.checked, toolName)
	if g.deny[toolName] {
		return false, "denied by test policy", nil
	}
	return true, "", nil
}

// TestGateBlocksDeniedCall proves executeOne consults the gate after the
// plan-mode check: a denied tool returns a "blocked:" result plus a notice and
// never runs, while an allowed tool runs normally.
func TestGateBlocksDeniedCall(t *testing.T) {
	reg := tool.NewRegistry()
	reg.Add(fakeTool{name: "bash", readOnly: false})
	reg.Add(fakeTool{name: "read_file", readOnly: true})

	g := &stubGate{deny: map[string]bool{"bash": true}}
	a := New(nil, reg, NewSession(""), Options{Gate: g}, event.Discard)

	blocked := a.executeOne(context.Background(), provider.ToolCall{Name: "bash", Arguments: `{"command":"rm -rf /"}`})
	if !strings.HasPrefix(blocked.output, "blocked:") {
		t.Errorf("denied call result = %q, want a 'blocked:' result", blocked.output)
	}
	if !blocked.blocked || blocked.errMsg == "" {
		t.Errorf("denied call should surface a user-facing block notice, got %+v", blocked)
	}

	ok := a.executeOne(context.Background(), provider.ToolCall{Name: "read_file", Arguments: `{"path":"/a"}`})
	if !strings.Contains(ok.output, "done") {
		t.Errorf("allowed call should run, got %q", ok.output)
	}

	if len(g.checked) != 2 {
		t.Errorf("gate consulted %d times, want 2 (%v)", len(g.checked), g.checked)
	}
}

// TestNilGateRunsEverything confirms gating is opt-in: with no gate wired, a
// writer call runs unimpeded (backward-compatible default).
func TestNilGateRunsEverything(t *testing.T) {
	reg := tool.NewRegistry()
	reg.Add(fakeTool{name: "write_file", readOnly: false})

	a := New(nil, reg, NewSession(""), Options{}, event.Discard) // no Gate
	out := a.executeOne(context.Background(), provider.ToolCall{Name: "write_file", Arguments: `{"path":"/a"}`})
	if strings.HasPrefix(out.output, "blocked:") {
		t.Errorf("nil gate should not block: %q", out.output)
	}
}

// stubGateApprover implements permission.Approver for tests.
type stubGateApprover struct {
	allow    bool
	remember bool
	err      error
	calls    int
}

func (s *stubGateApprover) Approve(ctx context.Context, tool, subject string, args json.RawMessage) (bool, bool, error) {
	s.calls++
	return s.allow, s.remember, s.err
}

// TestGatePreCheckSameTurnReRead verifies that PreCheck intercepts a second
// read_file for the same path within a single turn. This is the end-to-end
// equivalent of the control.TestReadFileTrackerSameTurnReRead test but
// exercises the real Agent → executeOne → Gate.Check → PreCheck path.
func TestGatePreCheckSameTurnReRead(t *testing.T) {
	reg := tool.NewRegistry()
	reg.Add(fakeTool{name: "read_file", readOnly: true})

	// Inline tracker: simulates readFileTracker.inTurn
	inTurn := make(map[string]bool)

	// Build a permission.Gate with PreCheck (like newInteractiveGate does).
	ap := &stubGateApprover{allow: true}
	g := permission.NewGate(permission.Policy{}, ap)
	g.PreCheck = func(toolName string, args json.RawMessage) (bool, string) {
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
		if inTurn[p.Path] {
			return true, "re-read " + p.Path + " — already in context"
		}
		inTurn[p.Path] = true
		return false, ""
	}

	a := New(nil, reg, NewSession(""), Options{Gate: g}, event.Discard)

	// First read: should not be blocked (PreCheck records it, returns false).
	out := a.executeOne(context.Background(), provider.ToolCall{
		Name: "read_file", Arguments: `{"path":"a.md","offset":0,"limit":500}`,
	})
	if out.blocked {
		t.Fatalf("first read should not be blocked, got blocked: %q", out.output)
	}
	if ap.calls != 0 {
		t.Fatalf("approver called %d times on first read, want 0", ap.calls)
	}

	// Second read of same path: PreCheck should return ask=true → approve.
	out = a.executeOne(context.Background(), provider.ToolCall{
		Name: "read_file", Arguments: `{"path":"a.md","offset":0,"limit":500}`,
	})
	if out.blocked {
		t.Fatalf("second read should be allowed by approver (allow=true), got blocked: %q", out.output)
	}
	if ap.calls != 1 {
		t.Fatalf("approver called %d times on second read, want 1 (PreCheck triggered)", ap.calls)
	}

	// Third scenario: approver denies → second read should be blocked.
	inTurn2 := make(map[string]bool)
	ap2 := &stubGateApprover{allow: false}
	g2 := permission.NewGate(permission.Policy{}, ap2)
	g2.PreCheck = func(toolName string, args json.RawMessage) (bool, string) {
		if toolName != "read_file" {
			return false, ""
		}
		var p struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal(args, &p); err != nil || p.Path == "" {
			return false, ""
		}
		if inTurn2[p.Path] {
			return true, "re-read " + p.Path + " — already in context"
		}
		inTurn2[p.Path] = true
		return false, ""
	}
	a2 := New(nil, reg, NewSession(""), Options{Gate: g2}, event.Discard)

	a2.executeOne(context.Background(), provider.ToolCall{
		Name: "read_file", Arguments: `{"path":"b.md"}`,
	})
	out = a2.executeOne(context.Background(), provider.ToolCall{
		Name: "read_file", Arguments: `{"path":"b.md"}`,
	})
	if !out.blocked {
		t.Fatal("second read should be blocked when approver denies")
	}
	if ap2.calls != 1 {
		t.Fatalf("approver called %d times on denied re-read, want 1", ap2.calls)
	}

	// Different path: should NOT trigger re-read check.
	inTurn3 := make(map[string]bool)
	ap3 := &stubGateApprover{allow: true}
	g3 := permission.NewGate(permission.Policy{}, ap3)
	g3.PreCheck = func(toolName string, args json.RawMessage) (bool, string) {
		if toolName != "read_file" {
			return false, ""
		}
		var p struct {
			Path string `json:"path"`
		}
		json.Unmarshal(args, &p)
		if inTurn3[p.Path] {
			return true, "re-read"
		}
		inTurn3[p.Path] = true
		return false, ""
	}
	a3 := New(nil, reg, NewSession(""), Options{Gate: g3}, event.Discard)

	a3.executeOne(context.Background(), provider.ToolCall{
		Name: "read_file", Arguments: `{"path":"c.md"}`,
	})
	a3.executeOne(context.Background(), provider.ToolCall{
		Name: "read_file", Arguments: `{"path":"d.md"}`,
	})
	if ap3.calls != 0 {
		t.Fatalf("approver called %d times for different paths, want 0", ap3.calls)
	}
}
