---
name: to-spec
description: Turn the current conversation into a spec document and save it as a Markdown file. This skill does NOT interview — it only synthesises what has already been discussed. Use after grill-with-docs or any sufficiently detailed conversation.
disable-model-invocation: true
---

# To Spec

Take the current conversation context and codebase understanding and produce a spec. **Do NOT interview the user** — just synthesise what you already know.

## Process

### 1. Explore the codebase (if not already done)

Understand the current state of the codebase. Use the project's domain glossary vocabulary from `CONTEXT.md` throughout the spec. Respect any ADRs in the area being touched.

### 2. Sketch the test seams

Identify the **seams** (interface boundaries) at which to test the feature:

- Existing seams should be preferred over new ones
- Use the highest seam possible (closest to the user-facing behaviour)
- The fewer seams across the codebase, the better — the ideal number is one

Check with the user that these seams match their expectations.

### 3. Write the spec

Use the template below. Save the spec as `docs/specs/<short-name>.md`. Create `docs/specs/` if it does not exist.

### 4. Done

Tell the user where the spec was saved and suggest running `/to-tickets` to break it into actionable tickets.

---

## Spec Template

### Problem Statement

The problem the user is facing, from the user's perspective.

### Solution

The solution to the problem, from the user's perspective.

### User Stories

A numbered list of user stories in the format:

> As an <actor>, I want <feature>, so that <benefit>

This list should be extensive and cover all aspects of the feature.

### Implementation Decisions

A list of decisions that were made, covering:

- The modules that will be built or modified
- The interfaces of those modules
- Technical clarifications
- Architectural decisions
- Schema changes
- API contracts

**Do NOT include specific file paths or code snippets** — they become outdated quickly.

Exception: if a prototype produced a snippet that encodes a decision more precisely than prose can (state machine, reducer, schema, type shape), inline it and note briefly that it came from a prototype. Trim to the decision-rich parts only.

### Testing Decisions

A list of testing decisions:

- What makes a good test (test external behaviour, not implementation details)
- Which modules will be tested and at which seams
- Prior art for the tests (similar tests already in the codebase)

### Out of Scope

What is explicitly not covered by this spec.

### Further Notes

Any additional context or caveats.
