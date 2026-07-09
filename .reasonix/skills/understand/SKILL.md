---
name: understand
description: Analyze the project codebase to produce an interactive knowledge graph for understanding architecture, components, and relationships. Run before exploring unfamiliar code.
argument-hint: "[path] [--full|--language <lang>]"
disable-model-invocation: true
---

# /understand

Analyze the current project to produce a `knowledge-graph.json` in `.understand-anything/`. The graph powers structural search and an interactive dashboard for exploring the project's architecture.

## Options

- `$ARGUMENTS` may contain:
  - `--full` — Force full rebuild, ignoring any existing graph
  - `--language <lang>` — Generate summaries/descriptions in the specified language (ISO 639-1, e.g. `zh`, `ja`, `en`). Default `en`.
  - A directory path (e.g. `../other-project`) — Analyze that directory instead of the current workspace

## Prerequisites

- Node.js ≥ 22 and pnpm ≥ 10 must be on PATH
- The Understand-Anything plugin must be at `OtherPackage/Understand-Anything/understand-anything-plugin/` (relative to the Reasonix repo root), or configured via `[understand_anything].plugin_path` in `reasonix.toml`

---

## Phase 0 — Pre-flight

### 0.1 Resolve project root

Set `PROJECT_ROOT`:
- If `$ARGUMENTS` contains a directory path, use that (resolved to absolute)
- Otherwise, use the current workspace root

### 0.2 Resolve plugin root

Set `PLUGIN_ROOT`:
- If `[understand_anything].plugin_path` is set in config, use it
- Otherwise, check `OtherPackage/Understand-Anything/understand-anything-plugin/` relative to the Reasonix repo root
- If neither exists, report: "Understand-Anything plugin not found. Set plugin_path in reasonix.toml or clone the repo." and STOP

### 0.3 Ensure core package is built

```bash
cd "$PLUGIN_ROOT" && (pnpm install --frozen-lockfile 2>/dev/null || pnpm install) && pnpm --filter @understand-anything/core build
```

### 0.4 Resolve skill scripts directory

Set `SKILL_DIR` to `$PLUGIN_ROOT/skills/understand/`.

### 0.5 Get current git commit

```bash
git -C "$PROJECT_ROOT" rev-parse HEAD
```
Store as `$GIT_COMMIT`.

### 0.6 Create working directories

```bash
mkdir -p "$PROJECT_ROOT/.understand-anything/intermediate"
mkdir -p "$PROJECT_ROOT/.understand-anything/tmp"
```

### 0.7 Decide: full vs incremental

- If `--full` flag is present → Full analysis
- If no existing `$PROJECT_ROOT/.understand-anything/knowledge-graph.json` → Full analysis
- If existing graph + unchanged commit hash → Ask: "The graph is up to date. (a) full rebuild, (b) do nothing?" Follow their choice. If (b), STOP.
- If existing graph + changed files → Incremental update

---

## Phase 1 — SCAN (full analysis only)

Report: `[Phase 1/7] Scanning project files...`

### 1.1 Collect project context

Read `README.md` (first 3000 chars) from `$PROJECT_ROOT` as `$README_CONTENT`.
Read the primary manifest (`package.json`, `go.mod`, `pyproject.toml`, etc.) as `$MANIFEST_CONTENT`.

Capture directory tree:
```bash
find "$PROJECT_ROOT" -maxdepth 2 -type f -not -path '*/node_modules/*' -not -path '*/.git/*' -not -path '*/dist/*' | head -100
```
Store as `$DIR_TREE`.

### 1.2 Run deterministic file scanner

```bash
node "$SKILL_DIR/scan-project.mjs" "$PROJECT_ROOT" "$PROJECT_ROOT/.understand-anything/intermediate/scan-result.json"
```

### 1.3 Extract import map

```bash
node "$SKILL_DIR/extract-import-map.mjs" "$PROJECT_ROOT" "$PROJECT_ROOT/.understand-anything/intermediate/scan-result.json" "$PROJECT_ROOT/.understand-anything/intermediate/import-map.json"
```

Read `$PROJECT_ROOT/.understand-anything/intermediate/scan-result.json` to get:
- Project name, description, languages, frameworks
- File list with `fileCategory` per file
- Complexity estimate

Store `importMap` as `$IMPORT_MAP` and file list as `$FILE_LIST`.

---

## Phase 1.5 — BATCH

Report: `[Phase 1.5/7] Computing semantic batches...`

```bash
node "$SKILL_DIR/compute-batches.mjs" "$PROJECT_ROOT"
```

Reads `scan-result.json`, writes `batches.json` to intermediate directory.

---

## Phase 2 — ANALYZE

Report: `[Phase 2/7] Analyzing files — <N> files in <M> batches...`

Load `$PROJECT_ROOT/.understand-anything/intermediate/batches.json`.

For each batch, use the `task` tool to dispatch a sub-agent that:
- Reads the file list for this batch
- Reads each file's content via `read_file`
- Produces `GraphNode` and `GraphEdge` objects following the UA schema (see Reference section below)
- Writes output to `$PROJECT_ROOT/.understand-anything/intermediate/batch-<i>.json`

**Run up to 4 batches concurrently.** For each `task` dispatch, include:
- The batch file list, languages, and import data from `batches.json[i].batchImportData`
- The schema reference from this SKILL.md
- The language directive (if `$OUTPUT_LANGUAGE` is not `en`)

After ALL batches complete, merge:

```bash
python "$SKILL_DIR/merge-batch-graphs.py" "$PROJECT_ROOT"
```

This produces `$PROJECT_ROOT/.understand-anything/intermediate/assembled-graph.json`.

---

## Phase 3 — ARCHITECTURE (layers)

Report: `[Phase 3/7] Identifying architectural layers...`

Use `task` to dispatch a sub-agent that:
- Reads the assembled graph
- Groups nodes into logical architectural layers (API, Business Logic, Data, Infrastructure, etc.)
- Writes to `$PROJECT_ROOT/.understand-anything/intermediate/layers.json`

Format: `[{ "id": "...", "name": "...", "description": "...", "nodeIds": [...] }]`

---

## Phase 4 — TOUR

Report: `[Phase 4/7] Generating guided tour...`

Use `task` to dispatch a sub-agent that:
- Reads the assembled graph and layers
- Generates an ordered tour of the project (5-10 steps)
- Writes to `$PROJECT_ROOT/.understand-anything/intermediate/tour.json`

Format: `[{ "order": 1, "title": "...", "description": "...", "nodeIds": [...] }]`

---

## Phase 5 — REVIEW & ASSEMBLE

Report: `[Phase 5/7] Validating knowledge graph...`

### 5.1 Run inline schema validation

```bash
node "$SKILL_DIR/extract-structure-result.mjs" "$PROJECT_ROOT/.understand-anything/intermediate/assembled-graph.json" "$PROJECT_ROOT/.understand-anything/intermediate/review.json"
```

### 5.2 Assemble final graph

Read the assembled graph, layers, and tour. Construct the final `KnowledgeGraph`:

```json
{
  "version": "1.0.0",
  "project": { "name": "...", "languages": [...], "frameworks": [...], "description": "...", "analyzedAt": "<ISO timestamp>", "gitCommitHash": "<hash>" },
  "nodes": [...],
  "edges": [...],
  "layers": [...],
  "tour": [...]
}
```

Apply automated fixes:
- Remove edges with dangling references
- Fill missing `tags` with `["untagged"]`
- Fill missing `summary` with `"No summary available"`
- Ensure every `layers[*].nodeIds` entry exists in nodes
- Ensure every `tour[*].nodeIds` entry exists in nodes

---

## Phase 6 — SAVE

Report: `[Phase 6/7] Saving knowledge graph...`

### 6.1 Write output files

Write the final graph to `$PROJECT_ROOT/.understand-anything/knowledge-graph.json`.

Write metadata to `$PROJECT_ROOT/.understand-anything/meta.json`:
```json
{
  "lastAnalyzedAt": "<ISO timestamp>",
  "gitCommitHash": "<current hash>",
  "version": "1.0.0",
  "analyzedFiles": <count>
}
```

### 6.2 Cleanup

```bash
TRASH="$PROJECT_ROOT/.understand-anything/.trash-$(date +%s)"
mkdir -p "$TRASH"
INTER="$PROJECT_ROOT/.understand-anything/intermediate"
if [ -d "$INTER" ]; then
  # Preserve scan-result.json for future incremental runs
  find "$INTER" -mindepth 1 -maxdepth 1 -not -name 'scan-result.json' -exec mv {} "$TRASH/" \; 2>/dev/null || true
fi
mv "$PROJECT_ROOT/.understand-anything/tmp" "$TRASH/" 2>/dev/null || true
```

### 6.3 Report summary

Report to the user:
- Nodes created (by type)
- Edges created (by type)
- Layers identified
- Tour steps generated
- Output path: `$PROJECT_ROOT/.understand-anything/knowledge-graph.json`

---

## Error Handling

- If any `task` sub-agent fails, retry **once** with the same prompt plus the failure context
- Track all warnings in `$PHASE_WARNINGS`
- If a phase fails twice, skip it and continue with partial results
- ALWAYS save partial results — a partial graph is better than no graph
- Report any skipped phases in the final summary

---

## Reference: KnowledgeGraph Schema

### Node Types (21 total)

| Type | Description | ID Convention |
|---|---|---|
| `file` | Source code file | `file:<relative-path>` |
| `function` | Function or method | `function:<relative-path>:<name>` |
| `class` | Class, interface, or type | `class:<relative-path>:<name>` |
| `module` | Logical module or package | `module:<name>` |
| `concept` | Abstract concept or pattern | `concept:<name>` |
| `config` | Configuration file | `config:<relative-path>` |
| `document` | Documentation file | `document:<relative-path>` |
| `service` | Service definition (Docker, etc.) | `service:<relative-path>` |
| `table` | Database table or view | `table:<relative-path>:<name>` |
| `endpoint` | API endpoint | `endpoint:<relative-path>:<name>` |
| `pipeline` | CI/CD pipeline | `pipeline:<relative-path>` |
| `schema` | Data schema (GraphQL, Proto) | `schema:<relative-path>` |
| `resource` | Infrastructure resource | `resource:<relative-path>` |

### Edge Types (26 total)

| Category | Types |
|---|---|
| Structural | `imports`, `exports`, `contains`, `inherits`, `implements` |
| Behavioral | `calls`, `subscribes`, `publishes`, `middleware` |
| Data flow | `reads_from`, `writes_to`, `transforms`, `validates` |
| Dependencies | `depends_on`, `tested_by`, `configures` |
| Semantic | `related`, `similar_to` |
| Infrastructure | `deploys`, `serves`, `provisions`, `triggers` |
| Schema/Data | `migrates`, `documents`, `routes`, `defines_schema` |

### Node fields

- `id` (string, required) — Unique identifier
- `type` (string, required) — One of the node types above
- `name` (string, required) — Human-readable name
- `filePath` (string, optional) — Relative file path
- `lineRange` ([number, number], optional) — Start/end line numbers
- `summary` (string, required) — Plain-English description
- `tags` (string[], required) — Searchable keywords
- `complexity` ("simple" | "moderate" | "complex", required)

### Edge fields

- `source` (string, required) — Source node ID
- `target` (string, required) — Target node ID
- `type` (string, required) — One of the edge types above
- `direction` ("forward" | "backward" | "bidirectional", required)
- `description` (string, optional)
- `weight` (number 0-1, required) — Importance of the relationship
