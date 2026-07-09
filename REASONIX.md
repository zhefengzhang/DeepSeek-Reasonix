# Reasonix project memory

This file is loaded into every session's system prompt (the cache-stable prefix),
so keep it concise and durable — it is the project's standing instructions to the
agent. It is the Reasonix analog of Claude Code's CLAUDE.md.

## Conventions

- Go kernel under `internal/`; each package owns one concern and documents it in a
  package comment. Match the surrounding comment density and idiom when editing.
- One transport-agnostic `control.Controller` sits behind every frontend (chat
  TUI, HTTP/SSE serve, Wails desktop). Add behavior to the controller, not a
  frontend, so all three inherit it.
- Cache-first: the system-prompt prefix (base prompt + tools + memory) must stay
  byte-stable across turns so DeepSeek's automatic prefix cache stays warm. Never
  mutate it mid-session — ride the turn tail instead (see `control.Compose`).

## Memory

- Hierarchical docs: `REASONIX.md` (this file, committed/shared), `REASONIX.local.md`
  (personal, git-ignored), user-global `~/.config/reasonix/REASONIX.md`, and any
  `REASONIX.md` in an ancestor dir. `AGENTS.md` is accepted as a fallback name.
- `@path` on its own line imports another file's contents.
- `#<note>` in chat quick-adds a line here. The `remember` tool saves durable
  facts to the per-project auto-memory store (frontmatter files + `MEMORY.md`
  index), which loads into the prefix on the next session.

## Engineering Skills

A set of composable playbooks lives at `.reasonix/skills/<name>/SKILL.md` and is discovered automatically by the skill store. Two invocation modes:

- **User-invoked** (`disable-model-invocation: true`) — type `/<name>` to activate. The agent cannot fire these on its own. Used for workflow orchestration.
- **Model-invoked** — the agent can reach for these autonomously when the task fits. Used for reusable engineering disciplines (TDD, code review, debugging).

Core workflow: `/grill-with-docs` (align) → `/to-spec` (spec) → `/to-tickets` (break down) → `/implement` (build with TDD) → `/code-review` (review). See `.reasonix/skills/00-INDEX.md` for the full list.

Skills are **not** system prompts — they are invoked on demand by the agent or the user. Their one-line index (name + description) is pinned in the cache-stable prefix; bodies load via `run_skill`.

**IMPORTANT: Use `run_skill` to invoke skills, NOT `slash_command`.** The `slash_command` tool handles only registered project commands, not skills from the skill store. The `run_skill` tool is the single entry point for all skills — call `run_skill({ name: "<skill-name>", arguments: "<task>" })`. User-invoked skills can also be triggered by the user typing `/<name>` in chat; model-invoked skills fire autonomously when the task fits.

## Pre-push CI simulation

Run these **before every commit** to catch the fastest CI failures locally:

```bash
gofmt -w .                          # catches gofmt (saves ~13s CI)
go vet ./...                        # catches vet warnings (saves ~52s CI/lint)
go test ./internal/tool/builtin/ ./internal/boot/  # catches tool/boot test breaks
```

CI runs `golangci-lint` (not locally available), but gofmt + vet already block ~80% of fast-fail scenarios.

## Import cycle rule

Before importing a new internal package from a non-test file, verify the target package's **test files** aren't already importing back to you:

```
# BAD: agent(_test.go) → tool/builtin(sessions.go) → agent  → setup failed
```

Use `go test ./path/to/target/` to detect cycles **before** pushing. A `[setup failed]` message means a cycle exists.

## PR hygiene

- **One force-push per round of review feedback.** Multiple force-pushes destroy review history and confuse reviewers.
- **Keep the PR diff minimal.** Only the files relevant to the PR's purpose — no stray changes from other branches.
- **Amend, don't add commits, for review feedback** — keeps the commit history clean.

## Cache-impact PR metadata

When PR changes touch files under `internal/boot/`, `internal/tool/`, `internal/provider/`, or other cache-sensitive paths (listed in `scripts/check-cache-impact.sh`), the PR body MUST include these lines at the end:

```
Cache-impact: <none|low|medium|high> — <reason>
Cache-guard: <focused guard test/command or existing guard rationale>
```

If the PR also touches files under `internal/config/`, `internal/memory/`, `internal/outputstyle/`, `internal/skill/`, or `internal/boot/`, add:

```
System-prompt-review: <reviewer/approval note>
```

Values `n/a`, `none`, `todo`, `tbd` are rejected — use a descriptive reason instead.
