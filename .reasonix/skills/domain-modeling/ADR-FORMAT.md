# ADR Format

> Architecture Decision Record: a short document capturing a decision that is hard to reverse, surprising without context, and the result of a real trade-off.

## Template

```markdown
# ADR NNNN: Title

## Status

Proposed | Accepted | Deprecated | Superseded by ADR-NNNN

## Context

What problem needed solving? What constraints were active? What options were considered?

## Decision

What was decided and why. Reference the key trade-off that tipped the balance.

## Consequences

What becomes easier, harder, or different because of this decision.
```

## Rules

1. Number sequentially: `0001-`, `0002-`, ...
2. Keep it short — three or four paragraphs max. If it is longer than one screen, the decision may be too broad.
3. Do not document decisions that are obvious or easily reversible.
4. A decision is ready for an ADR only when **all three** are true:
   - Hard to reverse
   - Surprising without context
   - The result of a real trade-off

## Done when

- A reader who was not in the conversation can understand why the choice was made
- The consequence section says what this decision unlocks or forecloses
