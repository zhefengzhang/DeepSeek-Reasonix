---
name: prototype
description: Build a throwaway prototype to answer a design question. Use when the user wants to sanity-check whether a state model or logic feels right, or explore what a UI should look like. Invoked automatically when a design question is hard to settle on paper.
---

# Prototype

A prototype is **throwaway code that answers a question**. The question decides the shape.

## Pick a branch

Identify which question is being answered:

- **"Does this logic / state model feel right?"** → build a tiny interactive terminal app that pushes the state machine through cases hard to reason about on paper.
- **"What should a Go API look like?"** → define the types and interfaces, write a minimal handler to demonstrate the ergonomics.

If the question is genuinely ambiguous, default to whichever better matches the surrounding code and state the assumption at the top.

## Rules

1. **Throwaway from day one.** Name the directory or file with a `_proto` suffix so a casual reader knows it is not production. Locate it close to where it will actually be used (next to the module it is prototyping for).
2. **One command to run.** `go run` or `go test -run` — the user must be able to start it without thinking.
3. **No persistence by default.** State lives in memory. Persistence is the thing the prototype is checking, not something it should depend on.
4. **Skip the polish.** No tests, no error handling beyond what makes the prototype runnable, no abstractions. The point is to learn something fast and then delete it.
5. **Surface the state.** After every action, print the full relevant state so the user can see what changed.
6. **Delete or absorb when done.** When the question is answered, either delete the prototype or fold the validated decision into the real code — do not leave it rotting in the repo.

## When done

The **answer** is the only thing worth keeping. Capture it somewhere durable (commit message, ADR, or a `NOTES.md` next to the prototype) along with the question it was answering. If the user is around, that capture is a quick conversation; if not, leave a placeholder so they (or you, on the next pass) can fill in the verdict before deleting.
