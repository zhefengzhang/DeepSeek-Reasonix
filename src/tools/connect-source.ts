/**
 * `connect_tool_source` — economy-mode on-demand capability activation.
 *
 * In economy mode, the agent boots with a minimal toolset to keep the
 * first-request prefix small. Optional capabilities (web search, skills,
 * MCP tools, plan/scaffold) are registered as dormant sources that the
 * model can activate on demand via this tool. Each activation costs one
 * cache-miss turn (the tool list changes), but subsequent requests benefit
 * from cache hits on the new prefix.
 */

import type { ToolRegistry } from "../tools.js";

export interface ConnectSourceOpts {
  /** Registry the source tools will be added to. */
  registry: ToolRegistry;
  /** All tool sources the model can activate. Keys are stable source names. */
  sources: Record<string, () => number>;
  /**
   * Called when a source is activated (after register). The callback
   * receives the source name and the number of tools registered so the
   * host can emit a notification / rebuild the prefix view.
   */
  onActivate?: (source: string, toolCount: number) => void;
}

export function registerConnectToolSource(
  registry: ToolRegistry,
  opts: ConnectSourceOpts,
): void {
  const available = Object.keys(opts.sources).filter(
    (k) => typeof opts.sources[k] === "function",
  );

  // Nothing to connect — skip registration entirely (no tool = no prefix bytes).
  if (available.length === 0) return;

  const sourceDesc = available.map((s) => `"${s}"`).join(", ");
  registry.register({
    name: "connect_tool_source",
    description: `Activate an optional tool source on demand (economy mode). Available sources: ${sourceDesc}. Call this when the current task requires capabilities not in the current toolset — the source's tools will become available starting from the next turn. Each activation costs one cache-miss turn.`,
    readOnly: true,
    parallelSafe: false,
    parameters: {
      type: "object",
      properties: {
        source: {
          type: "string",
          description: `Source to activate. Available: ${sourceDesc}.`,
          enum: available,
        },
      },
      required: ["source"],
    },
    fn: async (args: { source?: unknown }) => {
      const source = typeof args.source === "string" ? args.source.trim() : "";
      if (!source) {
        return JSON.stringify({
          error: `connect_tool_source requires a 'source' argument. Available sources: ${available.join(", ")}`,
        });
      }
      const activate = opts.sources[source];
      if (!activate) {
        return JSON.stringify({
          error: `unknown source ${JSON.stringify(source)}. Available: ${available.join(", ")}`,
        });
      }
      try {
        const count = activate();
        opts.onActivate?.(source, count);
        return JSON.stringify({
          ok: true,
          source,
          toolsActivated: count,
          note:
            count > 0
              ? `${count} tool(s) from "${source}" are now available. The next request will be a cache miss (the tool list changed), but subsequent requests will hit the cache again.`
              : `Source "${source}" was activated but registered 0 tools — it may have already been loaded.`,
        });
      } catch (err) {
        return JSON.stringify({
          error: `activate ${JSON.stringify(source)} failed: ${(err as Error).message}`,
        });
      }
    },
  });
}
