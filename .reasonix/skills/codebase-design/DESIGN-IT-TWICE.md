# Design It Twice

> Reference for `codebase-design`. When faced with a non-trivial interface design, design it several radically different ways before picking one.

## Why

The first design that comes to mind is almost never the best. It is shaped by whatever you last saw, whatever is fashionable, or whatever is most familiar. The second (or third) design surfaces trade-offs the first hid.

## How

Consider the interface from orthogonal angles. For each angle, sketch a full interface:

1. **By data flow** — push vs pull. Does the caller push data in, or pull results out?
2. **By lifecycle** — one-shot vs streaming. Does this module handle one request or a long-lived connection?
3. **By error handling** — return errors vs panic-and-recover. Does the caller handle errors inline, or catch them at a boundary?
4. **By coupling** — deep vs shallow. What is the highest-leverage seam you can place?

## Compare

For each candidate, evaluate:

- **Depth** — how much behaviour per unit of interface?
- **Locality** — does a change concentrate in one place or fan out?
- **Testability** — can you test through the interface without internal knowledge?

Pick the one that scores highest across all three. If two are tied, pick the one that is simpler to explain.

## Done when

- At least two radically different interface sketches exist
- Each sketch has a brief evaluation against depth, locality, and testability
- A winner is chosen with the reasoning documented
