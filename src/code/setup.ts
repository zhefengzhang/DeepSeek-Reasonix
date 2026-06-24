import { DeepSeekClient } from "../client.js";
import {
  type EditMode,
  loadEditMode,
  loadEndpoint,
  loadFilesystemOutlineThresholdBytes,
  loadGlobalShellAllowed,
  loadJavaSourceEnabled,
  loadProjectShellAllowed,
  loadResolvedSkillPaths,
  loadSubagentModels,
  loadToolRateLimit,
  readConfig,
  searchEnabled,
} from "../config.js";
import { bootstrapSemanticSearchInCodeMode } from "../index/semantic/tool.js";
import { ToolRegistry } from "../tools.js";
import { registerChoiceTool } from "../tools/choice.js";
import { registerCodeQueryTools } from "../tools/code-query.js";
import { registerConnectToolSource } from "../tools/connect-source.js";
import { registerFilesystemTools } from "../tools/filesystem.js";
import { registerJavaSourceTool } from "../tools/java-source.js";
import { JobRegistry } from "../tools/jobs.js";
import { registerMemoryTools } from "../tools/memory.js";
import { registerPlanTool } from "../tools/plan.js";
import { registerScaffoldTools } from "../tools/scaffold.js";
import { registerShellTools } from "../tools/shell.js";
import { type SkillInstalledHook, registerSkillTools } from "../tools/skills.js";
import {
  SHARED_SUBAGENT_SINK,
  type SubagentSink,
  formatSubagentResult,
  spawnSubagent,
} from "../tools/subagent.js";
import { registerTodoTool } from "../tools/todo.js";
import { registerWebTools } from "../tools/web.js";

export interface CodeToolsetOpts {
  rootDir: string;
  /** Override the default `~/.reasonix/config.json` lookup — primarily for tests that pin a tmp config. */
  configPath?: string;
  /** Fired after `install_skill` writes a new skill — desktop wires this to push a fresh `$skills` event so the sidebar updates without a tab reload. */
  onSkillInstalled?: SkillInstalledHook;
  /** Fired after `run_background` / `stop_job` mutate the JobRegistry — desktop pushes a fresh `$jobs` event so the popover updates without waiting for poll. */
  onJobsChanged?: () => void;
  /** Shared `{current: callback}` sink the TUI populates after mount. Setup forwards it into every `spawnSubagent` so live progress events reach the rich subagent row even though setup runs before the UI does. */
  subagentSink?: SubagentSink;
  /**
   * Token economy mode.
   * - `"full"` (default): all tools registered at boot — standard behavior.
   * - `"economy"`: only core coding tools at boot. Optional tool sources
   *   (web, skills, plan/scaffold, java, code-query, memory) are registered
   *   as dormant and activated on demand via `connect_tool_source`. Each
   *   activation costs one cache-miss turn but reduces the first-request
   *   prefix by up to 20K+ tokens.
   */
  tokenMode?: "full" | "economy";
}

export interface CodeToolset {
  tools: ToolRegistry;
  jobs: JobRegistry;
  registerRooted: (root: string) => void;
  reBootstrapSemantic: (root: string) => Promise<{ enabled: boolean }>;
  semantic: { enabled: boolean };
}

/** Mirror `editMode === "plan"` into the registry's dispatch gate — keeps a single source of truth (the persisted EditMode) for the read-only mode. */
export function applyPlanMode(tools: ToolRegistry, editMode: EditMode): void {
  tools.setPlanMode(editMode === "plan");
}

export async function buildCodeToolset(opts: CodeToolsetOpts): Promise<CodeToolset> {
  const tools = new ToolRegistry({ rateLimit: loadToolRateLimit() });
  applyPlanMode(tools, loadEditMode(opts.configPath));
  const jobs = new JobRegistry();

  const outlineThresholdBytes = loadFilesystemOutlineThresholdBytes();
  const registerRooted = (root: string): void => {
    // Core tools — always registered, economy or full.
    registerFilesystemTools(tools, {
      rootDir: root,
      outlineThresholdBytes,
      autoGitRollback: {},
    });
    const cfg = readConfig(opts.configPath);
    registerShellTools(tools, {
      rootDir: root,
      extraAllowed: () => [
        ...new Set([
          ...loadGlobalShellAllowed(opts.configPath),
          ...loadProjectShellAllowed(root, opts.configPath),
        ]),
      ],
      allowAll: () => loadEditMode(opts.configPath) === "yolo",
      jobs,
      onJobsChanged: opts.onJobsChanged,
      sensitivePaths: cfg.sensitivePaths,
    });
    // In economy mode, memory + code-query are optional sources activated on demand.
    // In full mode, registered immediately below.
  };

  const reBootstrapSemantic = async (root: string): Promise<{ enabled: boolean }> => {
    const result = await bootstrapSemanticSearchInCodeMode(tools, root);
    if (!result.enabled) tools.unregister("semantic_search");
    return result;
  };

  // Phase 1: Register core tools (always-on, economy + full).
  registerRooted(opts.rootDir);
  registerTodoTool(tools);

  // Phase 2: Optional tool sources. In economy mode these are wrapped in
  // deferred activation functions and wired through connect_tool_source.
  // In full mode they're registered immediately (current behavior).

  // Deferred subagent client — avoids throwing when DEEPSEEK_API_KEY is
  // unset at boot (setup wizard hasn't prompted yet).
  let subagentClient: DeepSeekClient | null = null;
  const buildSubagentRunner = (): import("../tools/skills.js").SubagentRunner => {
    return async (skill, task, signal) => {
      if (!subagentClient) {
        const ep = loadEndpoint();
        subagentClient = new DeepSeekClient({ apiKey: ep.apiKey, baseUrl: ep.baseUrl });
      }
      const result = await spawnSubagent({
        client: subagentClient,
        parentRegistry: tools,
        parentSignal: signal,
        system: skill.body,
        task,
        model: skill.model,
        allowedTools: skill.allowedTools,
        skillName: skill.name,
        sink: opts.subagentSink ?? SHARED_SUBAGENT_SINK,
      });
      return formatSubagentResult(result);
    };
  };

  // Builder for skill tools — called once in full mode, deferred in economy mode.
  const buildSkills = (): number => {
    const before = tools.specs().length;
    registerSkillTools(tools, {
      projectRoot: opts.rootDir,
      customSkillPaths: loadResolvedSkillPaths(opts.rootDir),
      subagentModels: loadSubagentModels(),
      onSkillInstalled: opts.onSkillInstalled,
      subagentRunner: buildSubagentRunner(),
    });
    return tools.specs().length - before;
  };

  const isEconomy = opts.tokenMode === "economy";

  if (isEconomy) {
    // Economy mode: core tools only at boot. Register connect_tool_source
    // with deferred activation functions for optional sources.
    const sources: Record<string, () => number> = {};

    // skills — run_skill + install_skill + built-in subagent wrappers
    sources["skills"] = buildSkills;

    // web — web_search + web_fetch
    if (searchEnabled()) {
      sources["web"] = () => {
        const before = tools.specs().length;
        registerWebTools(tools);
        return tools.specs().length - before;
      };
    }

    // plan — submit_plan + revise_plan + ask_choice
    sources["plan"] = () => {
      const before = tools.specs().length;
      registerPlanTool(tools);
      registerChoiceTool(tools);
      return tools.specs().length - before;
    };

    // scaffold — scaffolding/boilerplate tools
    sources["scaffold"] = () => {
      const before = tools.specs().length;
      registerScaffoldTools(tools, { projectRoot: opts.rootDir });
      return tools.specs().length - before;
    };

    // java_source — Java source analysis
    if (loadJavaSourceEnabled()) {
      sources["java_source"] = () => {
        const before = tools.specs().length;
        registerJavaSourceTool(tools, { projectRoot: opts.rootDir });
        return tools.specs().length - before;
      };
    }

    // memory + code_query — registered root-bound but deferred in economy
    sources["memory"] = () => {
      const before = tools.specs().length;
      registerMemoryTools(tools, { projectRoot: opts.rootDir });
      return tools.specs().length - before;
    };

    sources["code_query"] = () => {
      const before = tools.specs().length;
      registerCodeQueryTools(tools, { rootDir: opts.rootDir });
      return tools.specs().length - before;
    };

    registerConnectToolSource(tools, { registry: tools, sources });
  } else {
    // Full mode: register everything immediately (current behavior).
    buildSkills();
    registerPlanTool(tools);
    registerChoiceTool(tools);
    registerScaffoldTools(tools, { projectRoot: opts.rootDir });
    if (searchEnabled()) {
      registerWebTools(tools);
    }
    if (loadJavaSourceEnabled()) {
      registerJavaSourceTool(tools, { projectRoot: opts.rootDir });
    }
    registerMemoryTools(tools, { projectRoot: opts.rootDir });
    registerCodeQueryTools(tools, { rootDir: opts.rootDir });
  }

  const semantic = await reBootstrapSemantic(opts.rootDir);

  return { tools, jobs, registerRooted, reBootstrapSemantic, semantic };
}
