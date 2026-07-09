---
name: implement
description: Build the work described by a spec or set of tickets, driving TDD at pre-agreed seams and closing with code review before committing. Invoke after to-tickets to work through the ticket frontier.
disable-model-invocation: true
---

# Implement

Implement the work described by a spec or tickets.

## Process

### 1. Pick a ticket from the frontier

The **frontier** is any ticket whose blockers are all done. Read the ticket's description and acceptance criteria from `.scratch/<feature>/tickets.md`. If no `tickets.md` exists, work from the spec or the conversation directly.

### 2. Explore and prefactor

Understand the codebase area you will touch. Read `CONTEXT.md` for vocabulary and check `docs/adr/` for relevant decisions. Look for prefactoring opportunities — "make the change easy, then make the easy change."

### 3. Run /tdd

Use `/tdd` at the pre-agreed seams. Each vertical slice goes through:
- **Red**: write a failing test
- **Green**: write the minimal code to pass it
- The `/code-review` skill handles refactoring — do not refactor inside the TDD loop

### 4. Run typechecking regularly

Run `go vet ./...` and `go build ./...` frequently to catch type errors early. Run single test files after each green step, not just the full suite at the end.

### 5. Run /code-review

Once the ticket's acceptance criteria are met, run `/code-review` to review the diff before committing.

### 6. Commit

Commit your work to the current branch. Use a descriptive commit message referencing the ticket.

## Completion criterion

- All acceptance criteria for the ticket are met
- Tests pass (both new and existing)
- `/code-review` has been run and any blocking issues resolved
- Changes are committed

When the ticket is done, pick the next ticket from the frontier. If no tickets remain, the feature is complete.
