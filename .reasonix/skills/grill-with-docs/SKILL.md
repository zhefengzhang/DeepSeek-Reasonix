---
name: grill-with-docs
description: A relentless interview to sharpen a plan or design before building. Unlike grill-me, this skill also creates and updates CONTEXT.md (shared glossary) and ADRs (architecture decisions) as you discuss. Start here for any code work.
disable-model-invocation: true
---

# Grill With Docs

Run a **grilling** session, using the **domain-modeling** skill.

## How it works

1. Run `/domain-modeling` first to load the project's existing glossary and ADR discipline. It sets up the shared language context.
2. Then conduct a grilling session — interview the user relentlessly about every aspect of the plan until a shared understanding is reached.

## Grilling protocol

- Ask questions **one at a time**, waiting for feedback on each before continuing. Multiple questions at once is bewildering.
- If a *fact* can be found by exploring the codebase, look it up rather than asking the user. The *decisions*, though, are the user's — put each one to them and wait for an answer.
- Walk down each branch of the design tree, resolving dependencies between decisions one by one. For each question, provide your recommended answer.
- Do not enact the plan until the user confirms a shared understanding has been reached.

## Completion criterion

- **Checkable**: The user has explicitly said they are aligned ("yes, let's proceed" / "we are aligned" / equivalent)
- **Exhaustive**: Every decision branch that the conversation opened has been resolved. No unresolved "we will figure that out later" on critical path items.
- **Trail**: `CONTEXT.md` is updated (or created) with any new or clarified terms. ADRs are written for decisions that meet the three criteria (hard to reverse, surprising, traded-off).

When done, the user can proceed to `/to-spec` (if a spec is needed) or directly to `/to-tickets` / `/implement`.

## Related

- For non-code alignment (no codebase to scan), use `/grill-me`.
- For the underlying interview loop, see `/domain-modeling` and the built-in grilling instructions.
