# CONTEXT.md Format

> Shared glossary for the project's domain language. Often lives at the repo root.
> Implementation details, specs, and scratch notes must **not** go here.

## Sections

### Domain glossary

One term per bullet:

```markdown
- **Term** — one-sentence definition. Related term, opposite term.
```

### Key concepts

Briefly describe the 2-5 concepts that someone new to the codebase must understand first. No implementation details.

---

## Example

```markdown
# Context: Course Video Manager

## Domain Glossary

- **Course** — a collection of sections. Published or draft.
- **Section** — a group of lessons within a course. Ordered.
- **Lesson** — a single video + text within a section. Can be "placeholder" (no file yet) or "real" (file exists on disk).
- **Materialization** — the process of turning a placeholder lesson into a real one (allocating a file on disk).
- **Materialization Cascade** — the chain reaction: when a lesson materializes, every lesson after it in the same section shifts its file path.
```

## Done when

- Every term the conversation uses is defined or explicitly deferred for later resolution
- The glossary contains zero implementation details
- No two terms overlap in meaning
