# Context: Reasonix AI Agent

> This file is the shared glossary for the Reasonix project. It is maintained by the `domain-modeling` skill during sessions. It contains **only** domain terminology — no implementation details, specs, or scratch notes.

## Domain Glossary

- **Reasonix** — the AI coding agent client. A Go program that runs a chat TUI, an HTTP/SSE server, and a Wails desktop frontend, all driving the same `control.Controller`.
- **Skill** — a named, described prompt body (`SKILL.md`) that an agent can invoke via `run_skill` or `/<name>`. Lives under `.reasonix/skills/<name>/`. Two modes: inline (body folds into turn) and subagent (isolated child loop).
- **User-invoked skill** — a skill with `disable-model-invocation: true`. Only reachable by typing `/<name>`. The human is the index.
- **Model-invoked skill** — a skill without `disable-model-invocation`. The agent can fire it autonomously (and the human can still type `/<name>`).
- **Controller** — the transport-agnostic session driver in `internal/control/`. Every frontend drives the same `Controller`.
- **C** - **Turn** — one user message + the model's response cycle.
- **System prompt prefix** — the cache-stable part of the system prompt. Must stay byte-stable across turns to keep DeepSeek's prefix cache warm.
- **Memory** — hierarchical docs (`REASONIX.md`, `REASONIX.local.md`, `~/.config/reasonix/REASONIX.md`) plus auto-memory store (frontmatter files + `MEMORY.md`).
- **Seam** — a place where you can alter behaviour without editing in that place. A module's interface boundary.
- **Tracer bullet** — a vertical slice through all layers (schema, API, logic, tests) that is demoable on its own. The unit of work in the ticket system.
- **Frontier** — the set of tickets whose blockers are all done, so they can be picked up next.
- **TDD** — red → green → refactor. Test-first development.
