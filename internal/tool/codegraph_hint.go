package tool

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
)

// CodeGraphHint wraps a read-only built-in tool (grep or read_file) and appends
// a concise efficiency hint to its output when the target path has a recognised
// source-code file extension. The hint encourages the model to prefer
// understand_search / codegraph_explore / code_index for structural code
// exploration, which typically return richer results in fewer tokens.
//
// The wrapper delegates Name, Description, Schema, ReadOnly, and SnipHint
// transparently to the inner tool so the rest of the system sees no change.
type CodeGraphHint struct {
	inner    Tool
	codeExts map[string]bool // recognised code file extensions (lowercase)
}

// NewCodeGraphHint wraps inner with a codegraph efficiency hint. The wrapper
// is a no-op (no hint appended) when the file extension is not a recognised
// source-code extension, so document reads (.md, .txt, etc.) are unaffected.
func NewCodeGraphHint(inner Tool) *CodeGraphHint {
	return &CodeGraphHint{
		inner:    inner,
		codeExts: codeExtensions(),
	}
}

// IsCodeGraphHint reports whether t is a *CodeGraphHint wrapper.
// Used to prevent double-wrapping in idempotent call scenarios.
func IsCodeGraphHint(t Tool) bool {
	_, ok := t.(*CodeGraphHint)
	return ok
}

// --- tool.Tool interface passthrough ---

func (w *CodeGraphHint) Name() string            { return w.inner.Name() }
func (w *CodeGraphHint) Description() string     { return w.inner.Description() }
func (w *CodeGraphHint) Schema() json.RawMessage { return w.inner.Schema() }
func (w *CodeGraphHint) ReadOnly() bool          { return w.inner.ReadOnly() }

// Execute calls the inner tool and appends an efficiency hint when the
// target path has a recognised source-code extension.
func (w *CodeGraphHint) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	result, err := w.inner.Execute(ctx, args)
	if err != nil {
		return result, err // never hint on errors
	}
	if !w.shouldHint(args) {
		return result, nil
	}
	return result + codegraphHintText, nil
}

// SnipHint delegates to the inner tool's SnipHinter when implemented,
// or returns a safe default. This is critical: the hint text lives at the
// tail end of the output, and the SnipHint tail budget must preserve it.
func (w *CodeGraphHint) SnipHint() SnipHint {
	if h, ok := w.inner.(SnipHinter); ok {
		return h.SnipHint()
	}
	return SnipHint{Head: 40, Tail: 10, HeadChars: 6000, TailChars: 1000}
}

// --- hint gating ---

// shouldHint returns true when the tool call targets source code (a recognised
// code extension, or a directory/project-level path we cannot rule out).
func (w *CodeGraphHint) shouldHint(args json.RawMessage) bool {
	ext := w.parseExtension(args)
	if ext == "" || ext == "." {
		// No extension found — treat as directory/project-level search. This
		// covers grep(".", "."), read_file("./internal/agent"), and empty paths.
		return true
	}
	return w.codeExts[ext]
}

// parseExtension extracts the file extension from the 'path' argument in the
// tool's JSON arguments. Returns "" for missing/empty paths and directories.
func (w *CodeGraphHint) parseExtension(args json.RawMessage) string {
	var p struct {
		Path string `json:"path"`
	}
	if len(args) == 0 {
		return ""
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return ""
	}
	path := strings.TrimSpace(p.Path)
	if path == "" || path == "." {
		return "" // project-level scope
	}
	return strings.ToLower(filepath.Ext(path))
}

// --- hint text ---

const codegraphHintText = "\n\n💡 This operation targeted source code. For code-structure exploration:\n" +
	"`codegraph_explore` returns verbatim source with call context in ~1-3K tokens.\n" +
	"A project-wide grep/read_file scan costs ~5-15K tokens with less structural context.\n" +
	"Priority: `understand_search` → `codegraph_explore` → `code_index` → `grep`/`read_file`."

// codeExtensions returns the set of recognised source-code file extensions
// (lowercase, with leading dot). Extensions NOT in this set are treated as
// non-code (documentation, config, data) and never receive the hint.
func codeExtensions() map[string]bool {
	return map[string]bool{
		".go": true, ".js": true, ".ts": true, ".jsx": true, ".tsx": true,
		".py": true, ".rs": true, ".java": true, ".kt": true, ".kts": true,
		".cs": true,
		".c":  true, ".cc": true, ".cpp": true, ".cxx": true,
		".h": true, ".hh": true, ".hpp": true, ".hxx": true,
		".m": true, ".mm": true,
		".swift": true, ".rb": true, ".php": true, ".scala": true,
		".clj": true, ".cljs": true, ".ex": true, ".exs": true,
		".erb": true, ".vue": true, ".svelte": true,
		".zig": true, ".lua": true, ".dart": true,
	}
}
