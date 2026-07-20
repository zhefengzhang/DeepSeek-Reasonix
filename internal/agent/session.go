// Package agent wires a Provider, a tool Registry, and a Session into the
// harness loop that drives a coding task to completion.
package agent

import (
	"sync"

	"reasonix/internal/provider"
)

// Session holds the conversation history for one task. The run loop (one turn at
// a time) is the only writer, but a frontend can read History/Save from another
// goroutine while a turn appends, so mu guards Messages. Direct Messages reads on
// the run-loop goroutine stay lock-free (serial with its own writes); cross-
// goroutine access goes through Snapshot.
type Session struct {
	mu             sync.RWMutex
	Messages       []provider.Message
	rewriteVersion int // bumped each time the log is rewritten (compact/fold)
	// version tracks tool-call preview updates. Separated from rewriteVersion
	// so a preview refresh after a mid-turn snapshot triggers a rewrite save
	// without conflating the compaction counter.
	version int
	// normalizedDirty is set when LoadSession repaired the history on the way in
	// (empty tool-call names, dangling calls, truncated args, …). The repair
	// already lives in Messages, so the next Save persists it automatically as
	// part of the usual full rewrite; the flag exists for observability and to
	// let callers opt out of work that a dirty session would make redundant.
	normalizedDirty bool
}

// NewSession initializes a session with an optional system prompt.
func NewSession(system string) *Session {
	s := &Session{}
	if system != "" {
		s.Messages = append(s.Messages, provider.Message{Role: provider.RoleSystem, Content: system})
	}
	return s
}

// Add appends a message.
func (s *Session) Add(m provider.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Messages = append(s.Messages, m)
}

// Replace swaps the whole message log — used by compaction, which rewrites the
// middle of the history.
func (s *Session) Replace(msgs []provider.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Messages = msgs
}

// Snapshot returns a copy of the messages, safe to read from another goroutine
// while a turn appends. Frontends (History, Save) use it instead of touching the
// live slice.
func (s *Session) Snapshot() []provider.Message {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]provider.Message(nil), s.Messages...)
}

// RewriteVersion returns the current rewrite version.
func (s *Session) RewriteVersion() int { return s.rewriteVersion }

// IncrementRewrite bumps the rewrite version by 1.
func (s *Session) IncrementRewrite() { s.rewriteVersion++ }

// NeedsRewriteSave reports whether the session log was mutated after a
// mid-turn Snapshot, so the caller must use SaveRewrite instead of an
// append-only SaveSnapshot to avoid duplicating the already-persisted
// assistant message.
func (s *Session) NeedsRewriteSave() bool {
	return s.rewriteVersion > 0 || s.version > 0
}

// SaveRewrite persists the full session and resets the rewrite markers. Use
// this after mid-turn snapshot mutations so the saved file replaces the
// original assistant message instead of appending tool results after it.
func (s *Session) SaveRewrite(path string) error {
	if err := s.Save(path); err != nil {
		return err
	}
	s.rewriteVersion = 0
	s.version = 0
	return nil
}

// UpdateToolCallPreview replaces the preview fields of the newest matching
// assistant tool call. A dependent writer can only be previewed after an
// earlier writer in the same model batch succeeds; updating under the session
// lock keeps live History/Snapshot readers race-free and ensures the refreshed
// preview is what a resumed session archives.
func (s *Session) UpdateToolCallPreview(call provider.ToolCall) bool {
	if call.ID == "" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := len(s.Messages) - 1; i >= 0; i-- {
		if s.Messages[i].Role != provider.RoleAssistant {
			continue
		}
		calls := s.Messages[i].ToolCalls
		for j := range calls {
			if calls[j].ID != call.ID {
				continue
			}
			cloned := append([]provider.ToolCall(nil), calls...)
			cloned[j].Diff = call.Diff
			cloned[j].Added = call.Added
			cloned[j].Removed = call.Removed
			s.Messages[i].ToolCalls = cloned
			// A snapshot may have persisted the original assistant message while
			// its tools were still running. Mark this as a rewrite so a later
			// autosave replaces that message instead of misclassifying the tool
			// results as an append-only suffix.
			s.version++
			return true
		}
	}
	return false
}

// HasContent returns true when the session carries at least one user,
// assistant, or tool message — i.e. more than just a system prompt. An
// "empty" conversation that has never been used should not be persisted.
func (s *Session) HasContent() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, m := range s.Messages {
		if m.Role != provider.RoleSystem {
			return true
		}
	}
	return false
}

// HasSystemMessage reports whether the session starts with a system message,
// which carries the agent's stable identity and behavioural contract. Sessions
// without one are not safe to persist: when reloaded the model has no identity
// context and falls back to its training-data defaults.
func (s *Session) HasSystemMessage() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.Messages) > 0 && s.Messages[0].Role == provider.RoleSystem
}
