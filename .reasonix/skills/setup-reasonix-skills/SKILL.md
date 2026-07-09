---
name: setup-reasonix-skills
description: One-time setup for the Reasonix engineering skills ecosystem. Creates required directories, explains the skill conventions, and verifies that the skills index is correctly discovered. Run once before first use of the other engineering skills.
disable-model-invocation: true
---

# Setup Reasonix Skills

Configure this project for the Reasonix engineering skills. This is a prompt-driven skill — explore, present what is found, confirm with the user, then set up.

## Process

### 1. Explore

Check the current state of the project:

- Does `.reasonix/skills/` exist? What skills are already present?
- Does `CONTEXT.md` exist at the project root?
- Does `docs/adr/` exist?
- Is `docs/specs/` set up?
- Is `.scratch/` available for ticket files?

### 2. Present findings and set up

Walk through these items one at a time, confirming with the user before each:

**Section A — Skills index.**
Verify that the skills are correctly placed in `.reasonix/skills/<name>/SKILL.md`. The next session will discover them automatically through Reasonix's skill discovery.

**Section B — Domain docs layout.**
Create `CONTEXT.md` if it does not exist, with a minimal template. Create `docs/adr/` if it does not exist. Explain that:
- `CONTEXT.md` holds the shared glossary (implementation details go elsewhere)
- `docs/adr/` holds architecture decision records
- Both are maintained by the `/domain-modeling` skill during sessions

**Section C — Workspace files.**
Create `docs/specs/` for spec documents (created by `/to-spec`). Ensure `.scratch/` exists for ticket files (created by `/to-tickets`). Create `.reasonix/handoffs/` for handoff documents (created by `/handoff`).

### 3. Confirm and write

Show the user a summary of what was done and which skills are now available.

## Completion criterion

- The required directories exist (or were already present)
- `CONTEXT.md` has a minimal template
- The user understands the skill invocation model (type `/name` for user-invoked, agent auto-fires model-invoked)
