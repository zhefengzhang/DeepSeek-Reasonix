# Deepening a Cluster Given Its Dependencies

> Reference for `codebase-design`. When a module cluster has known dependencies, this guide helps you deepen it — more behaviour behind the same or smaller interface.

## Dependency Categories

Classify each dependency of the module cluster:

| Category | Definition | Example |
|----------|-----------|---------|
| **Infrastructure** | I/O, network, filesystem, clock | Postgres connection, HTTP client, `os.File` |
| **Domain** | Core business logic types | `Order`, `User`, `Invoice` |
| **Orchestration** | Coordinates other modules | Workflow engine, saga runner, pipeline |
| **Utility** | Stateless helpers | String formatting, math, hashing |

## Seam Discipline

1. **Put seams at category boundaries.** Infrastructure dependencies get adapters. Domain dependencies get interfaces defined by the consumer. Orchestration gets events or callbacks. Utilities get direct calls (no seam needed).
2. **Do not seam within a category.** An interface for every domain type is over-engineering, not depth.
3. **Let the caller define the seam.** The module's interface should be what the caller needs, not what the implementation happens to expose.

## Replace-don't-layer Testing

When testing a deep module:

1. **Replace dependencies at the seam.** Pass a fake adapter instead of the real Postgres client.
2. **Do not add a thin wrapper "just for testing."** If you need a wrapper to test, the seam is in the wrong place — move it to a real boundary.
3. **Test through the interface, not around it.** If you need to set internal state, the interface is missing something.

## Done when

- Every infrastructure dependency has an adapter at a seam
- No domain type has a dedicated seam unless two implementations genuinely exist
- The module can be tested by replacing only its infrastructure adapters
