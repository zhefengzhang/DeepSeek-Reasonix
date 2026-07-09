---
name: grill-me
description: A relentless interview to sharpen a plan or design. Unlike grill-with-docs, this skill does not write CONTEXT.md or ADRs — it is a pure conversation. Use for non-code scenarios (project planning, process design, architecture without a codebase) or quick alignment that does not need documentation.
disable-model-invocation: true
---

# Grill Me

Conduct a **grilling** session — interview the user relentlessly about every aspect of the plan until a shared understanding is reached.

## Protocol

- Ask questions **one at a time**, waiting for feedback on each before continuing. Multiple questions at once is bewildering.
- If a *fact* can be found by exploring the current context, look it up rather than asking. The *decisions* are the user's — put each one to them and wait.
- Walk down each branch of the decision tree, resolving dependencies between decisions one by one. For each question, provide your recommended answer.
- Do not enact the plan until the user confirms a shared understanding has been reached.

## When to use this instead of grill-with-docs

| Situation | Use |
|-----------|-----|
| You have a codebase and want docs out of the session | `/grill-with-docs` |
| You are planning a non-code project (writing, process, design) | `/grill-me` |
| You want a quick alignment on something small before coding | `/grill-me` |
| You want shared vocabulary to persist across sessions | `/grill-with-docs` |

## Completion criterion

The user has explicitly confirmed alignment — "yes, let's go" or equivalent. No unresolved decision branches remain.
