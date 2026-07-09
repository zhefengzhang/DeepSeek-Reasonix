---
name: improve-codebase-architecture
description: Scan the codebase for deepening opportunities — places where a module could be deeper (more behaviour behind a smaller interface). Present findings as a report, then work through one candidate at a time. Run every few days to prevent code entropy.
disable-model-invocation: true
---

# Improve Codebase Architecture

Scan the codebase for **deepening opportunities** — modules that could be deeper (more behaviour behind a smaller interface). Present the findings, then grill through whichever one you pick.

## Process

### 1. Scan

Walk the `internal/` tree and look for:

- **Shallow modules** — thin wrappers around other packages where the interface is as complex as the implementation. Apply the deletion test: if you deleted this module, would the complexity vanish or reappear across callers?
- **Missing seams** — hardcoded dependencies on infrastructure (direct Postgres calls in business logic, file I/O in handlers) that could be abstracted behind an interface.
- **Leaky abstractions** — modules that expose implementation details in their interface (exposed config structs, exported internals, error types from specific libraries).
- **God packages** — packages with too many responsibilities that could be split.

### 2. Report

Write the findings to `ARCHITECTURE-NOTES.md` in the repo root (or update it if it exists). For each candidate, show:

- The module path
- What kind of opportunity it is (shallow / missing seam / leaky abstraction / god package)
- A one-sentence diagnosis
- The estimated depth gain (more leverage? more locality? both?)

### 3. Grill

Present the report to the user and ask which candidate to tackle. Use `/codebase-design` vocabulary throughout the discussion. Once a candidate is chosen, use `/grill-with-docs` to align on the design before implementing.

### Completion criterion

The report is written. If the user picks a candidate, the design is discussed and ready for `/implement`.
