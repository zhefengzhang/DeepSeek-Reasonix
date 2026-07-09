---
name: code-review
description: Two-axis code review of the diff since a fixed point — Standards (does it follow coding standards plus design smell baseline?) and Spec (does it faithfully implement the originating issue/spec?). Invoked automatically by implement before committing.
---

# Code Review

Two-axis review of the changes since a fixed point (usually the base branch). Run **Standards** and **Spec** as parallel concerns so neither pollutes the other.

## Scope

Default scope: the current diff against the default branch (`git diff main...HEAD` or equivalent). If a specific commit range or file set is named, honor that instead.

Read touched files when the diff alone lacks context — signatures, surrounding invariants, callers.

## Axis 1: Standards

Check the diff against:

1. **Coding conventions** — does the code follow the project's established patterns? Naming, error handling, package organisation, test style. Reference `REASONIX.md` and `CONTRIBUTING.md` if they exist.
2. **Design smells** (Fowler baseline) — look for:
   - Shotgun surgery (one change touches many files)
   - Divergent change (one file changes for many reasons)
   - Feature envy (a method that seems more interested in another class than its own)
   - Inappropriate intimacy (two modules that know too much about each other)
   - Refused bequest (inheritance that throws away most of what it inherits)
3. **Test quality** — do the tests verify behaviour through public interfaces? Are they readable? Do they avoid implementation coupling?

## Axis 2: Spec

Check the diff against the originating spec, issue, or ticket:

1. **Does it implement what was specified?** Every user story or acceptance criterion should be addressed.
2. **Are there behaviours that were NOT specified but were added?** Flag them — they may be scope creep or necessary discoveries.
3. **Are there specified behaviours that are missing?** Flag them as gaps.

## Output

Lead with a one-sentence verdict: "ship as-is" / "minor nits, OK to ship after" / "blocking issues, do not ship".

Then a short bulleted list, each with:
- file:line
- The problem in one sentence
- What to change

Group by severity if more than 4 items: **Blocking** → **Should-fix** → **Nits**.

If everything looks clean, say so plainly. Do not manufacture concerns.

## Completion criterion

Every blocking issue is resolved before commit. Should-fix and nits are documented for the author's awareness but do not block.
