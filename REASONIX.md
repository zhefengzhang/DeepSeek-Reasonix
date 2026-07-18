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

## Code Intelligence

**⚠️ HARD RULE: Every task MUST begin with `understand_search` FIRST, then
`codegraph_explore` (`mcp__codegraph__codegraph_explore`) SECOND. grep and
read_file are LAST RESORT. Violating this workflow multiplies token cost
by 5-15×. The tools are free to call — skipping them is negligence.**

Two complementary tools serve different granularities. **Using the right one
first saves dozens of token-heavy tool calls.** Every task MUST start with the
appropriate code intelligence tool — never grep or read_file to answer a
question either tool can answer faster.

### Decision tree

| You need to… | Tool | Rationale | Estimated cost |
|---|---|---|---|
| Find what module / file / layer owns X | `understand_search` | Knowledge graph already indexed the project skeleton. One search returns the node id you need. | ~0.5–1K tokens |
| See the **verbatim source** of a symbol (function, type, component) | `codegraph_explore` (`mcp__codegraph__codegraph_explore`) | Returns Read-equivalent line-numbered source + call paths + blast radius in **one capped call**. Treat the output as already Read — do NOT re-open those files. | ~1–3K tokens |
| Trace a **cross-file flow** (who calls X, what X calls, callbacks, React re-render hops) | `codegraph_explore` | Dynamic-dispatch edges (JSX children, observer callbacks, React setState→render) are bridged automatically. grep can't follow these. | ~2–4K tokens |
| Find exact text / regex across files | `grep` | Fallback only. Use AFTER the graph tools tell you which files matter. | ~5–15K tokens for a project-wide scan |
| Read a file you already identified | `read_file` | Use only after `codegraph_explore` tells you which file and lines matter. | ~2–5K tokens per file |

Choosing `grep` first when `codegraph_explore` would do: 5–15× more tokens for less structural context.

### Mandatory workflow

1. **`understand_search` FIRST** — locate the module, file, or architectural layer. Never skip this step; the knowledge graph is cache-stable and costs zero prefix tokens.
2. **`codegraph_explore` SECOND** — pass the symbol names (or a natural-language question) from step 1. One call returns the source you would otherwise grep + Read across 5-15 files.
3. **`grep` / `read_file` only as fallback** — when both graph tools are unavailable, or for exact string searches neither covers.

### Anti-patterns (will waste tokens and lead to cache misses)

- ❌ `grep` for a symbol name the knowledge graph already indexes.
- ❌ `read_file` on a file `codegraph_explore` already returned source for.
- ❌ Skipping `understand_search` and diving into grep directly.
- ❌ Using `glob` for structural questions — use `understand_search outline` instead.

## Memory

- Hierarchical docs: `REASONIX.md` (this file, committed/shared), `REASONIX.local.md`
  (personal, git-ignored), user-global `~/.config/reasonix/REASONIX.md`, and any
  `REASONIX.md` in an ancestor dir. `AGENTS.md` is accepted as a fallback name.
- `@path` on its own line imports another file's contents.
- `#<note>` in chat quick-adds a line here. The `remember` tool saves durable
  facts to the per-project auto-memory store (frontmatter files + `MEMORY.md`
  index), which loads into the prefix on the next session.

## Engineering Discipline

### Before any task

1. **Check the graph** — `understand_search` first to locate modules, then
   `codegraph_explore` for verbatim source. Never grep before the graph tells
   you which files matter.
2. **Anchor impact** — for bugs: users affected, what's blocked, any workaround?
   For features: how do we verify success after launch?
3. **Anchor symptoms** — precise, not vague. "P99 rose from 200ms→3s during
   peak" not "system is slow."
4. **Find root cause** — drill down through code changes, data thresholds,
   dependency timeouts, config changes, resource quotas. Bottom layer.
5. **Narrowest fix** — answer: why it solves the root cause? Can it be one line?
   Does it exceed scope? Does it introduce new problems? What's the rollback?
6. **Verify** — normal path, edge cases, all dependents, regression suite. Then
   mentally simulate "how does this fail in production?"
7. **Close the loop** — how did this escape tests? Add a test case, update a
   checklist, or add a scanning rule.

### Before declaring done

Ask yourself these eight. One unanswered, don't deliver.

1. Did I check the graph before touching code?
2. Did I reproduce the issue locally?
3. Did I overstep scope? Read the diff line by line — any non-essential line
   without its own issue number must be reverted.
4. Did regression pass on all upstream callers and downstream dependents?
5. Most likely production failure mode? Rollback plan?
6. If this breaks again tomorrow, can a new person locate the cause from logs?
7. Did this fix leave a process improvement? (test, checklist, scanning rule)
8. Does the feature have clear acceptance criteria? Who confirms it succeeded?

### Scope discipline

Forbidden without an independent issue:
- Refactoring an entire function while fixing one bug in it.
- Renaming or restyling a related module while adding a feature.
- Changing a global config other modules depend on.
- Rewriting code because it "looks bad" though the task doesn't touch it.
- Introducing a new dependency or tool because "it's more elegant."

Allowed: cleanup within 3 lines of the change. Beyond that needs its own issue.

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
