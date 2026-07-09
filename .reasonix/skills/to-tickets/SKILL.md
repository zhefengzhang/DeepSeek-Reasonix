---
name: to-tickets
description: Break a plan, spec, or conversation into a set of tracer-bullet tickets, each declaring its blocking edges. Tickets are written as local Markdown files under .scratch/<feature>/tickets.md. Use after to-spec.
disable-model-invocation: true
---

# To Tickets

Break a plan, spec, or conversation into a set of **tickets** — tracer-bullet vertical slices, each declaring the tickets that **block** it.

Tickets are written to local Markdown files since this project uses a local-markdown issue tracker. See `.scratch/<feature>/tickets.md`.

## Process

### 1. Gather context

Work from whatever is in the conversation context. If the user passes a reference (a spec path), read it.

### 2. Explore the codebase (optional)

If you have not already explored the codebase, do so. Ticket titles and descriptions should use the project's domain glossary vocabulary from `CONTEXT.md`. Respect ADRs in the area being touched.

Look for opportunities to **prefactor** the code to make implementation easier. "Make the change easy, then make the easy change."

### 3. Draft vertical slices

Break the work into **tracer bullet** tickets — each slice cuts a narrow but **complete** path through every layer (schema, API, UI, tests).

Rules:

- Each slice is **vertical** — cuts through all layers, NOT a horizontal slice of one layer
- A completed slice is **demoable or verifiable** on its own
- Each slice is sized to fit in a single fresh context window
- Any prefactoring should be done first (as its own ticket)

Give each ticket its **blocking edges** — the other tickets that must complete before it can start. A ticket with no blockers can start immediately.

**Wide refactors are the exception.** A wide refactor (one mechanical change with a blast radius across the whole codebase) cannot be a vertical slice. Sequence it as **expand–contract**:
1. **Expand**: add the new form beside the old, breaking nothing
2. **Migrate**: migrate call sites in batches sized by blast radius
3. **Contract**: delete the old form once no caller remains

### 4. Quiz the user

Present the proposed breakdown. For each ticket, show:

- **Title**: short descriptive name
- **Blocked by**: which tickets must complete first (or "None — can start immediately")
- **What it delivers**: the end-to-end behaviour this ticket makes work

Ask the user:

- Does the granularity feel right? (too coarse / too fine)
- Are the blocking edges correct?
- Should any tickets be merged or split further?

Iterate until the user approves the breakdown.

### 5. Write the tickets file

Write the approved tickets to `.scratch/<short-name>/tickets.md`. Create the directory if needed.

---

## Tickets File Template

```markdown
# Tickets: <short name>

A one-line summary of what these tickets build. Reference the source spec if there is one.

Work the **frontier**: any ticket whose blockers are all done.

## <Ticket 1>

**What to build:** the end-to-end behaviour this ticket makes work, from the user's perspective.

**Blocked by:** None — can start immediately.

- [ ] Acceptance criterion 1
- [ ] Acceptance criterion 2

## <Ticket 2>

**What to build:** ...

**Blocked by:** <Ticket 1>

- [ ] ...
```

## Done when

- All tickets are written to `.scratch/<feature>/tickets.md`
- The user has approved the breakdown
- Each ticket has clear acceptance criteria
- Blocking edges are explicit
