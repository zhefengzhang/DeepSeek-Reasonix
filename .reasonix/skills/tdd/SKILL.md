---
name: tdd
description: Test-driven development with a red-green-refactor loop. Use when building features or fixing bugs test-first. Invoked automatically when implement or other build skills need the TDD discipline.
---

# Test-Driven Development

TDD red → green loop. This skill is **reference** for producing tests worth keeping: what a good test is, where tests go, anti-patterns, and the rules of the loop. Every section applies on every cycle — consult them before and during the loop, not after.

When exploring the codebase, read `CONTEXT.md` (if it exists) and test names to match the project's domain vocabulary. Respect ADRs in the area being touched.

## What a good test is

Tests verify behaviour through **public interfaces**, not implementation details. Code can be entirely rewritten; tests should not change. A good test reads like a specification — "user can checkout a valid cart" tells you exactly what capability exists. It survives refactors and does not care about internal structure.

## Seams — where tests go

A **seam** is the public boundary at which to test: an interface where you observe behaviour without reaching inside. Tests live at seams, never against internals.

**Test only pre-agreed seams.** Before writing any test, confirm the seams with the user. No test is written at an unconfirmed seam. Not everything can be tested — agreeing seams up front ensures testing effort lands on critical paths and complex logic, not every edge case.

Ask: "What is the public interface? Which seams should we test?"

## Anti-patterns

- **Implementation-coupled** — mocks internal collaborators, tests private methods, verifies through a side channel. Tell: the test breaks when you refactor.
- **Self-referential expected values** — the test copies the implementation's own logic as the expected value. Expected values must come from an independent source of truth: a known-good literal, a worked example, or the spec.
- **Horizontal slicing** — writing all tests first, then all implementation. Bulk tests verify *imagined* behaviour. Work in **vertical slices** instead: one test → one implementation → repeat.

## Rules of the loop

1. **Red before green.** Write a failing test first, then only enough code to pass it. Do not anticipate future tests or add speculative features.
2. **One slice at a time.** One seam, one test, one minimal implementation per cycle.
3. **Refactoring is not part of the loop.** Refactoring belongs in the review stage (see `code-review` skill), not the red → green implementation cycle.
4. **Run the test after every green step.** Confirm the new code does not break existing tests. Run `go test ./...` once at the end.
5. **Keep the test suite fast.** If a test takes more than a few seconds to run, it is probably testing at the wrong seam — push it closer to the logic.

## Completion criterion

A failing test is now passing with the minimal implementation. The next cycle can begin (or the seam is complete if all behaviours are covered).
