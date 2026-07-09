---
name: handoff
description: Compact the current conversation into a handoff document for another agent session to continue. Saves to .reasonix/handoffs/<timestamp>-<name>.md. Use when the context window is getting full, or when branching off into a prototype session.
disable-model-invocation: true
argument-hint: "What will the next session be used for?"
---

# Handoff

Write a handoff document summarising the current conversation so a fresh agent can continue the work. Save to `.reasonix/handoffs/<timestamp>-<name>.md`.

## Content

Include in the handoff:

1. **What was accomplished** — a brief summary of the current state
2. **What remains** — the next steps, unresolved questions, and decisions still pending
3. **Key decisions made** — reference any ADRs, specs, or `tickets.md` that capture the details
4. **Suggested skills** — which skills the next session should invoke (e.g. `/implement`, `/tdd`, `/grill-with-docs`)
5. **Artifact references** — paths to specs, tickets, commits, or diffs

## Rules

- Do not duplicate content already captured in other artifacts (specs, plans, ADRs, issues, commits). Reference them by path or URL instead.
- Redact sensitive information (API keys, passwords, PII).
- If the user passed arguments, treat them as a description of what the next session will focus on and tailor the document accordingly.

## Completion criterion

The handoff document exists at `.reasonix/handoffs/<timestamp>-<name>.md`. The user is told where it was saved and which skills the next session should use.
