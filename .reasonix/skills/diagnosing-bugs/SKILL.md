---
name: diagnosing-bugs
description: Disciplined diagnosis loop for hard bugs and performance regressions. Invoked automatically when a bug resists a quick fix or the user asks for structured debugging.
---

# Diagnosing Bugs

A disciplined diagnosis loop for bugs that resist a first glance, intermittent flakes, or regressions between two known-good states. Do not theorise until you have a **tight feedback loop** — one command that already goes **red** on this bug.

## The loop

### 1. Reproduce

Get a reliable reproduction. Do not skip this step. If you cannot reproduce, you cannot confirm a fix.

- Write a test that fails due to the bug (red)
- Or craft a shell command that demonstrates the wrong behaviour
- The reproduction must be **deterministic** — it fails every time when the bug is present

### 2. Minimise

Narrow the bug to its minimal trigger:

- Binary search: comment out half the code, see if the bug persists
- Reduce inputs: find the smallest input that triggers the bug
- Isolate the layer: is it in the handler, the service, the data layer?

### 3. Hypothesise

Form a specific hypothesis about the root cause, not a vague direction:

- ❌ "Something is wrong with the auth flow"
- ✅ "The JWT token expiry is not being checked on the admin route because the middleware skips validation for paths starting with /admin/v1"

### 4. Instrument

Add logging, print statements, or debugger output to confirm or refute the hypothesis. One hypothesis at a time.

### 5. Fix

Apply the fix. Run the reproduction from step 1 to confirm it is now **green**.

### 6. Regression test

Ensure the fix is captured in a regression test so the same bug cannot reappear:

- The reproduction from step 1 **is** the regression test if it is automated
- If the reproduction was a manual command, convert it into an automated test

## When done

- The bug is fixed and confirmed green by the reproduction
- A regression test exists
- If the diagnosis revealed a design problem (no good seam to lock the bug down), consider running `/improve-codebase-architecture`

## Completion criterion

The reproduction from step 1 now passes. A regression test is committed with the fix.
