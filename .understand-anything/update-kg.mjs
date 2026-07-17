#!/usr/bin/env node
/**
 * update-kg.mjs — 离线知识图谱增量更新脚本
 *
 * 用法:
 *   cd <project-root>
 *   node .reasonix/update-kg.mjs
 *
 * 前置依赖:
 *   - Node.js >= 22
 *   - Understand-Anything plugin 位于 OtherPackage/Understand-Anything/understand-anything-plugin/
 *   - @understand-anything/core 已构建
 *   - 已有 .understand-anything/knowledge-graph.json（AI 生成的基线）
 *
 * 流程:
 *   detectChanges() → extractStructures() → transformToGraphNodes() → mergeIntoGraph() → rebuildLayers() → save()
 *
 * 不依赖任何 AI 调用。
 * 使用 extract-structure.mjs（TreeSitter）进行结构化提取，
 * 确定性规则生成 summary/tags/complexity。
 */

import { createRequire } from 'node:module';
import { dirname, resolve, relative, join } from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';
import { existsSync, readFileSync, writeFileSync, mkdirSync, renameSync, unlinkSync } from 'node:fs';
import { execFileSync } from 'node:child_process';

// ========================================================================
// 配置
// ========================================================================

const PROJECT_ROOT = resolve(import.meta.dirname, '..');
const PLUGIN_RELATIVE = 'OtherPackage/Understand-Anything/understand-anything-plugin';
const PLUGIN_ROOT = resolve(PROJECT_ROOT, PLUGIN_RELATIVE);
const SKILL_DIR = resolve(PLUGIN_ROOT, 'skills/understand');
const INTERMEDIATE_DIR = resolve(PROJECT_ROOT, '.understand-anything/intermediate');
const GRAPH_PATH = resolve(PROJECT_ROOT, '.understand-anything/knowledge-graph.json');
const META_PATH = resolve(PROJECT_ROOT, '.understand-anything/meta.json');

// 复杂度行数阈值
const COMPLEXITY_THRESHOLDS = [
  { max: 50, label: 'simple' },
  { max: 200, label: 'moderate' },
  { max: Infinity, label: 'complex' },
];

// 层级映射规则：路径前缀 → 层级名
const LAYER_RULES = [
  { prefix: 'desktop/frontend/src/components/', name: 'Desktop/frontend/src/components' },
  { prefix: 'desktop/frontend/src/lib/', name: 'Desktop/frontend/src/lib' },
  { prefix: 'desktop/frontend/src/__tests__/', name: 'Desktop/frontend/src/__tests__' },
  { prefix: 'desktop/frontend/src/', name: 'Desktop/frontend/src/lib' },
  { prefix: 'desktop/frontend/', name: 'Desktop' },
  { prefix: 'desktop/', name: 'Desktop' },
  { prefix: 'internal/agent/', name: 'Internal/agent' },
  { prefix: 'internal/cli/', name: 'Internal/cli' },
  { prefix: 'internal/tool/builtin/', name: 'Internal/tool/builtin' },
  { prefix: 'internal/control/', name: 'Internal/control' },
  { prefix: 'internal/config/', name: 'Internal/config' },
  { prefix: 'internal/provider/', name: 'Internal/config' },
  { prefix: 'internal/skill/', name: 'Internal/config' },
  { prefix: 'internal/understand/', name: 'Internal/config' },
  { prefix: 'internal/boot/', name: 'Internal/cli' },
  { prefix: 'internal/', name: 'Internal/cli' },
  { prefix: 'docs/', name: 'Docs' },
];

// ========================================================================
// Understand-Anything 核心模块懒加载
// ========================================================================

/** 缓存 createIgnoreFilter 的引用 */
let _ignoreFilterFn = null;

async function loadCreateIgnoreFilter() {
  if (_ignoreFilterFn) return _ignoreFilterFn;
  try {
    const require_ = createRequire(resolve(PLUGIN_ROOT, 'package.json'));
    const core = await import(pathToFileURL(require_.resolve('@understand-anything/core')).href);
    _ignoreFilterFn = core.createIgnoreFilter;
    return _ignoreFilterFn;
  } catch (err) {
    console.warn(`[update-kg] Could not load .understandignore filter: ${err.message} — skipping filtering`);
    return null;
  }
}

/**
 * 使用 .understandignore 规则过滤文件列表。
 * 当无法加载过滤模块或项目没有 .understandignore 时，返回原列表。
 */
async function filterIgnoredFiles(filePaths, projectRoot) {
  const rootIgnorePath = join(projectRoot, '.understandignore');
  if (!existsSync(rootIgnorePath)) {
    return filePaths; // 没有 .understandignore，不过滤
  }

  const createFilter = await loadCreateIgnoreFilter();
  if (!createFilter) {
    return filePaths; // 过滤模块不可用，不过滤
  }

  const filter = createFilter(projectRoot);
  const kept = [];
  let dropped = 0;
  for (const p of filePaths) {
    if (filter.isIgnored(p)) {
      dropped++;
    } else {
      kept.push(p);
    }
  }
  if (dropped > 0) {
    console.log(`[update-kg] .understandignore filtered ${dropped} file(s)`);
  }
  return kept;
}

// ========================================================================
// 工具函数
// ========================================================================

/** 执行 git 命令并返回 stdout 去除末尾换行 */
function git(args) {
  return execFileSync('git', args, { cwd: PROJECT_ROOT, encoding: 'utf-8' }).trimEnd();
}

/** 计算文件复杂度 */
function deriveComplexity(lines) {
  for (const t of COMPLEXITY_THRESHOLDS) {
    if (lines <= t.max) return t.label;
  }
  return 'complex';
}

/** 从文件名推导语言标签 */
function deriveLanguageTags(language, path) {
  const tags = [language];
  if (language === 'go') tags.push('go');
  else if (language === 'typescript' || language === 'javascript' || language === 'tsx') tags.push('typescript');
  else if (language === 'css') tags.push('css');
  else if (language === 'markdown') tags.push('documentation');
  if (/\.(test|spec)\.(tsx?|jsx?)$/.test(path) || path.endsWith("_test.go") || path.includes("/__tests__/")) tags.push("test");
  if (path.includes('/desktop/')) tags.push('desktop');
  if (path.startsWith('internal/')) tags.push('internal');
  if (path.startsWith('desktop/frontend/')) tags.push('frontend');
  return tags;
}

/** 从相对路径推导文件的职责摘要 */
function deriveFileSummary(filePath, language, totalLines, metrics, functions) {
  const basename = filePath.split('/').pop();
  const dir = filePath.split('/').slice(0, -1).join('/');
  const funcNames = (functions || []).map(f => f.name);
  const isTest = basename.match(/\.(test|spec|_test)\./) || basename.endsWith('_test.go');
  const funcCount = metrics?.functionCount ?? (functions || []).length;
  const lineCount = totalLines || 0;

  if (isTest) {
    const prodFile = basename
      .replace(/\.(test|spec)\./, '.')
      .replace(/_test\./, '.');
    const testFuncs = funcNames.filter(n => n.startsWith('Test')).slice(0, 3);
    const suffix = testFuncs.length > 0 ? ` covering ${testFuncs.join(', ')}${testFuncs.length < funcNames.filter(n=>n.startsWith('Test')).length ? ', ...' : ''}` : '';
    return `Test suite for ${prodFile}${suffix}`;
  }

  // 关键词匹配
  const lowerName = basename.toLowerCase();
  const lowerDir = dir.toLowerCase();

  const patterns = [
    { match: 'favorites', result: 'Favorites management module' },
    { match: 'bridge', result: 'Wails bridge between frontend and Go kernel' },
    { match: 'store', result: 'State store' },
    { match: 'settingspanel', result: 'Desktop settings panel' },
    { match: 'app.tsx', result: 'Root React application component' },
    { match: 'approvalmodal', result: 'Tool/plan approval modal component' },
    { match: 'composer', result: 'Chat composer component' },
    { match: 'transcript', result: 'Chat transcript component' },
    { match: 'message', result: 'Chat message components' },
    { match: 'layout', result: 'Desktop shell layout store' },
    { match: 'styles.css', result: 'Global CSS styles and theme variables' },
    { match: 'types.ts', result: 'Central TypeScript type definitions' },
    { match: /locales\/\w+\.ts$/, result: 'UI i18n translation dictionary' },
    { match: /headroom_sidecar/, result: 'Headroom proxy sidecar lifecycle manager' },
    { match: 'tabs', result: 'Workspace tab management module' },
    { match: /branch/, result: 'Branch/session metadata sidecar manager' },
    { match: 'controller', result: 'Transport-agnostic session controller' },
    { match: /effort/, result: 'Effort level configuration logic' },
    { match: /edit_test/, result: 'Config edit test suite' },
    { match: /effort_test/, result: 'Effort config test suite' },
    { match: /boot_test/, result: 'Boot sequence test suite' },
    { match: /favorites_app/, result: 'Favorites management module' },
    { match: /understand_search/, result: 'Knowledge graph search tool' },
    { match: /prefix/, result: 'System prompt prefix generator' },
  ];

  for (const { match: pattern, result: desc } of patterns) {
    if (typeof pattern === 'string' && (lowerName.includes(pattern) || lowerDir.includes(pattern))) {
      return `${desc}: ${funcCount} function(s), ${lineCount} lines`;
    }
    if (pattern instanceof RegExp && pattern.test(filePath)) {
      return `${desc}: ${funcCount} function(s), ${lineCount} lines`;
    }
  }

  // —— 路径推导 ——
  let category = '', langLabel = language;
  if (language === 'typescript') langLabel = 'TypeScript';
  else if (language === 'go') langLabel = 'Go';
  else if (language === 'tsx') langLabel = 'TypeScript/React';

  if (filePath.startsWith('desktop/frontend/src/components/'))
    category = 'React UI component';
  else if (filePath.startsWith('desktop/frontend/src/lib/'))
    category = 'Frontend library module';
  else if (filePath.startsWith('desktop/frontend/src/store/'))
    category = 'Frontend state store';
  else if (filePath.startsWith('desktop/frontend/src/locales/'))
    category = 'UI i18n translation file';
  else if (filePath.startsWith('desktop/frontend/src/__tests__/'))
    category = 'Frontend test';
  else if (filePath.startsWith('desktop/frontend/'))
    category = 'Frontend asset/config';
  else if (filePath.startsWith('desktop/'))
    category = 'Desktop Go module';
  else if (filePath.startsWith('internal/agent/'))
    category = 'Agent core module';
  else if (filePath.startsWith('internal/cli/'))
    category = 'CLI module';
  else if (filePath.startsWith('internal/tool/builtin/'))
    category = 'Built-in tool';
  else if (filePath.startsWith('internal/control/'))
    category = 'Session controller';
  else if (filePath.startsWith('internal/config/') || filePath.startsWith('internal/provider/') || filePath.startsWith('internal/skill/') || filePath.startsWith('internal/understand/'))
    category = 'Config/provider module';
  else if (filePath.startsWith('internal/boot/'))
    category = 'Boot/initialization module';
  else if (filePath.startsWith('internal/'))
    category = 'Internal module';
  else if (filePath.startsWith('docs/'))
    category = 'Documentation';
  else
    category = 'Miscellaneous';

  return `${langLabel} ${category}: ${basename} — ${funcCount} function(s), ${lineCount} lines`;
}

/** 从结构数据推导函数摘要 */
function deriveFunctionSummary(fn, filePath) {
  const paramCount = (fn.params || []).length;
  const isExported = /^[A-Z]/.test(fn.name);
  const prefix = isExported ? 'Exported' : 'Unexported';
  const paramDesc = paramCount === 0 ? 'no parameters' : `${paramCount} parameter(s)`;

  const nameLower = fn.name.toLowerCase();
  const fileLower = filePath.toLowerCase();
  const patterns = [
    { match: 'favorite', result: 'favorites management' },
    { match: 'start', result: 'process/component startup' },
    { match: 'stop', result: 'process/component shutdown' },
    { match: 'save', result: 'persistence/save operation' },
    { match: 'load', result: 'load/deserialize operation' },
    { match: 'new', result: 'constructor/factory' },
    { match: 'get', result: 'read/getter operation' },
    { match: 'set', result: 'write/setter operation' },
    { match: 'test', result: 'test case' },
    { match: 'build', result: 'build/assemble operation' },
  ];
  let context = '';
  for (const { match: pat, result: desc } of patterns) {
    if (nameLower.includes(pat) && fileLower.includes(pat)) {
      context = ` — ${desc}`;
      break;
    }
  }
  if (!context) {
    const base = filePath.split('/').pop().replace(/\.(go|tsx?|mjs)$/, '');
    context = ` in ${base}`;
  }

  return `${prefix} function ${fn.name} with ${paramDesc}${context}`;
}

/** 推导文件标签 */
function deriveFileTags(filePath, language, metrics) {
  const tags = deriveLanguageTags(language, filePath);
  if (filePath.startsWith('desktop/frontend/')) tags.push('react');
  if (filePath.includes('store/') && language === 'typescript') tags.push('zustand');
  if (filePath.includes('bridge') || filePath.endsWith('bridge.ts')) tags.push('wails');
  if (metrics?.functionCount > 15) tags.push('complex');
  else if (metrics?.functionCount > 5) tags.push('moderate');
  else tags.push('simple');
  return [...new Set(tags)];
}

/** 归一化边方向：lowercase、别名映射、无效值兜底 */
function normalizeDirection(dir) {
  if (!dir) return 'forward';
  const d = dir.toLowerCase().trim();
  if (d === 'both' || d === 'mutual' || d === 'any') return 'bidirectional';
  if (d === 'backward' || d === 'reverse') return 'backward';
  return 'forward';
}

// ========================================================================
// 核心函数
// ========================================================================

/** 1. 读取配置 */
function readConfig() {
  let meta = null;
  let graph = null;

  if (existsSync(META_PATH)) {
    meta = JSON.parse(readFileSync(META_PATH, 'utf-8'));
    console.log(`[update-kg] Last analysis: commit ${meta.gitCommitHash}, ${meta.analyzedFiles} files`);
  }

  if (existsSync(GRAPH_PATH)) {
    graph = JSON.parse(readFileSync(GRAPH_PATH, 'utf-8'));
    console.log(`[update-kg] Existing graph: ${graph.nodes?.length || 0} nodes, ${graph.edges?.length || 0} edges, ${graph.layers?.length || 0} layers`);
  }

  return { meta, graph };
}

/** 2. 检测 Git 变更 */
function detectChanges(lastCommitHash, fullMode) {
  const currentHead = git(['rev-parse', 'HEAD']);
  console.log(`[update-kg] Current HEAD: ${currentHead}`);

  if (fullMode) {
    // 全量模式：从 scan-result.json 获取所有文件
    const scanPath = resolve(INTERMEDIATE_DIR, 'scan-result.json');
    let allFiles = [];
    if (existsSync(scanPath)) {
      const scanData = JSON.parse(readFileSync(scanPath, 'utf-8'));
      // 只选代码文件（跳过 .gitignore 忽略的）
      allFiles = (scanData.files || []).filter(f => f.fileCategory === 'code').map(f => f.path);
    }
    if (allFiles.length === 0) {
      // 兜底：直接 git ls-files
      allFiles = git(['ls-files']).split('\n').filter(f => f.trim());
    }
    console.log(`[update-kg] Full mode: will analyze ${allFiles.length} files`);
    return { changed: allFiles, newFiles: allFiles, deletedFiles: [], currentHead };
  }

  if (!lastCommitHash) {
    console.log('[update-kg] No prior analysis — use --full to rebuild from scratch.');
    return { changed: [], newFiles: [], deletedFiles: [], currentHead };
  }

  if (currentHead === lastCommitHash) {
    console.log('[update-kg] Graph is up to date. No changes since last analysis.');
    return { changed: [], newFiles: [], deletedFiles: [], currentHead };
  }

  const allChanged = git(['diff', '--name-only', `${lastCommitHash}..HEAD`])
    .split('\n')
    .filter(f => f.trim() && !f.startsWith('.understand-anything/'));

  const deletedLines = git(['diff', '--diff-filter=D', '--name-only', `${lastCommitHash}..HEAD`])
    .split('\n')
    .filter(f => f.trim());
  const deletedSet = new Set(deletedLines);
  const deletedFiles = allChanged.filter(f => deletedSet.has(f));
  const newFiles = git(['diff', '--diff-filter=A', '--name-only', `${lastCommitHash}..HEAD`])
    .split('\n')
    .filter(f => f.trim() && !f.startsWith('.understand-anything/'));
  const modifiedFiles = allChanged.filter(f => !deletedSet.has(f) && !newFiles.includes(f));

  console.log(`[update-kg] Changes from ${lastCommitHash}: ${modifiedFiles.length} modified, ${newFiles.length} added, ${deletedFiles.length} deleted`);

  return { changed: allChanged, newFiles, deletedFiles, currentHead };
}

/** 3. 收集需删除的旧节点 */
function collectOldNodeIDs(graph, changedFiles, deletedFiles) {
  const allAffected = [...new Set([...changedFiles, ...deletedFiles])];
  const removeIDs = new Set();
  const changedFileIDs = new Set(allAffected.map(f => `file:${f}`));

  for (const node of graph.nodes || []) {
    if (changedFileIDs.has(node.id)) {
      removeIDs.add(node.id);
      continue;
    }
    const nid = node.id;
    if (nid.includes(':')) {
      const first = nid.indexOf(':');
      const rest = nid.slice(first + 1);
      for (const af of allAffected) {
        if (rest === af || rest.startsWith(af + ':')) {
          removeIDs.add(nid);
          break;
        }
      }
    }
  }

  console.log(`[update-kg] Nodes to remove: ${removeIDs.size}`);
  return removeIDs;
}

/** 4. 调用 extract-structure.mjs 获取结构数据 */
async function extractStructures(changedFiles) {
  if (changedFiles.length === 0) return [];

  const scanPath = resolve(INTERMEDIATE_DIR, 'scan-result.json');
  let scanData = null;
  if (existsSync(scanPath)) {
    scanData = JSON.parse(readFileSync(scanPath, 'utf-8'));
  }
  const fileMeta = new Map();
  if (scanData?.files) {
    for (const f of scanData.files) {
      fileMeta.set(f.path, f);
    }
  }

  const batchFiles = changedFiles.map(f => {
    const meta = fileMeta.get(f) || {};
    return {
      path: f,
      language: meta.language || guessLanguage(f),
      sizeLines: meta.sizeLines || 0,
      fileCategory: meta.fileCategory || guessCategory(f),
    };
  });

  if (!existsSync(SKILL_DIR)) {
    console.error(`[update-kg] ERROR: Skill dir not found at ${SKILL_DIR}`);
    return [];
  }

  mkdirSync(INTERMEDIATE_DIR, { recursive: true });

  const inputPath = resolve(INTERMEDIATE_DIR, 'kg-extract-input.json');
  const outputPath = resolve(INTERMEDIATE_DIR, 'kg-extract-output.json');

  const inputData = {
    projectRoot: PROJECT_ROOT,
    batchFiles,
    batchImportData: {},
  };

  writeFileSync(inputPath, JSON.stringify(inputData, null, 1), 'utf-8');

  try {
    const extractScript = resolve(SKILL_DIR, 'extract-structure.mjs');
    console.log(`[update-kg] Running extract-structure.mjs on ${batchFiles.length} file(s)...`);
    execFileSync('node', [extractScript, inputPath, outputPath], {
      cwd: SKILL_DIR,
      encoding: 'utf-8',
      timeout: 300_000,
    });

    if (!existsSync(outputPath)) {
      console.error('[update-kg] extract-structure.mjs produced no output');
      return [];
    }

    const result = JSON.parse(readFileSync(outputPath, 'utf-8'));
    console.log(`[update-kg] Extraction: ${result.filesAnalyzed} analyzed, ${result.filesSkipped?.length || 0} skipped`);
    return result.results || [];
  } catch (err) {
    console.error(`[update-kg] extract-structure.mjs failed:`, err.message);
    return [];
  }
}

/** 5. 结构数据 → GraphNode 和 GraphEdge */
function transformToGraphNodes(extractResults, changedFiles) {
  const newNodes = [];
  const newEdges = [];
  const changedSet = new Set(changedFiles);

  for (const result of extractResults) {
    const filePath = result.path;
    if (!filePath || !changedSet.has(filePath)) continue;

    const metrics = result.metrics || {};
    const totalLines = result.totalLines || 0;
    const language = result.language || guessLanguage(filePath);
    const functions = result.functions || [];
    const classes = result.classes || [];
    const definitions = result.definitions || [];
    const exports = result.exports || [];

    // 文件节点
    const fileNode = {
      id: `file:${filePath}`,
      type: 'file',
      name: filePath.split('/').pop(),
      summary: deriveFileSummary(filePath, language, totalLines, metrics, functions),
      tags: deriveFileTags(filePath, language, metrics),
      complexity: deriveComplexity(totalLines),
    };
    newNodes.push(fileNode);

    // 函数节点
    for (const fn of functions) {
      if (!fn.name) continue;
      const fnID = `function:${filePath}:${fn.name}`;
      newNodes.push({
        id: fnID,
        type: 'function',
        name: fn.name,
        summary: deriveFunctionSummary(fn, filePath),
        tags: deriveLanguageTags(language, filePath),
        complexity: 'simple',
      });
      newEdges.push({ source: fileNode.id, target: fnID, type: 'contains', direction: 'forward', weight: 1.0 });
    }

    // 类/类型节点
    const classMap = new Map();
    for (const cls of classes) {
      if (!cls.name) continue;
      const clsID = `class:${filePath}:${cls.name}`;
      classMap.set(clsID, cls);
      newNodes.push({
        id: clsID, type: 'class', name: cls.name,
        summary: `Class with ${(cls.methods || []).length} method(s) and ${(cls.properties || []).length} propertie(s)`,
        tags: deriveLanguageTags(language, filePath),
        complexity: (cls.methods || []).length > 5 ? 'moderate' : 'simple',
      });
      newEdges.push({ source: fileNode.id, target: clsID, type: 'contains', direction: 'forward', weight: 1.0 });
    }

    // Go 结构体 / TS 定义
    for (const def of definitions) {
      if (!def.name) continue;
      const defID = `class:${filePath}:${def.name}`;
      if (classMap.has(defID)) continue;
      newNodes.push({
        id: defID, type: 'class', name: def.name,
        summary: `${def.kind || 'struct'} definition with ${(def.fields || []).length} field(s)`,
        tags: deriveLanguageTags(language, filePath),
        complexity: (def.fields || []).length > 8 ? 'moderate' : 'simple',
      });
      newEdges.push({ source: fileNode.id, target: defID, type: 'contains', direction: 'forward', weight: 1.0 });
    }

    // 导出边
    const exportedNames = new Set(exports.map(e => e.name));
    for (const fn of functions) {
      if (exportedNames.has(fn.name)) {
        newEdges.push({ source: fileNode.id, target: `function:${filePath}:${fn.name}`, type: 'exports', direction: 'forward', weight: 0.7 });
      }
    }

    // tested_by 边：测试文件→被测文件（统一 forward 方向）
    if (filePath.endsWith('_test.go') || filePath.match(/\.(test|spec)\.(ts|tsx)$/)) {
      const prodPath = filePath
        .replace(/_test\.go$/, '.go')
        .replace(/\.(test|spec)\.(ts|tsx)$/, '.$2');
      const testFileID = `file:${filePath}`;
      const prodFileID = `file:${prodPath}`;
      newEdges.push({
        source: testFileID,
        target: prodFileID,
        type: 'tested_by',
        direction: normalizeDirection('forward'),
        weight: 0.5,
      });
    }
  }

  console.log(`[update-kg] Transform: ${newNodes.length} nodes, ${newEdges.length} edges`);
  return { newNodes, newEdges };
}

/** 6. 合并到图谱 */
function mergeIntoGraph(graph, removeIDs, newNodes, newEdges) {
  if (!graph.nodes) graph.nodes = [];
  if (!graph.edges) graph.edges = [];
  if (!graph.layers) graph.layers = [];

  if (removeIDs.size > 0) {
    graph.nodes = graph.nodes.filter(n => !removeIDs.has(n.id));
    graph.edges = graph.edges.filter(e => !removeIDs.has(e.source) && !removeIDs.has(e.target));
    console.log(`[update-kg] Removed old: ${graph.nodes.length} nodes remain`);
  }

  // 添加或刷新节点
  const existingByID = new Map(graph.nodes.map(n => [n.id, n]));
  let addedNodes = 0;
  let refreshedNodes = 0;
  for (const n of newNodes) {
    const old = existingByID.get(n.id);
    if (old) {
      const oldKey = JSON.stringify(old);
      const newKey = JSON.stringify(n);
      if (oldKey !== newKey) {
        Object.assign(old, n);
        refreshedNodes++;
      }
    } else {
      graph.nodes.push(n);
      existingByID.set(n.id, n);
      addedNodes++;
    }
  }
  if (refreshedNodes > 0) {
    console.log(`[update-kg] Refreshed ${refreshedNodes} existing node(s) with updated content`);
  }

  const existingEdges = new Set(graph.edges.map(e => `${e.source}|${e.target}|${e.type}`));
  let addedEdges = 0;
  for (const e of newEdges) {
    const key = `${e.source}|${e.target}|${e.type}`;
    if (!existingEdges.has(key)) {
      graph.edges.push(e);
      existingEdges.add(key);
      addedEdges++;
    }
  }

  console.log(`[update-kg] Merged: +${addedNodes} nodes, +${addedEdges} edges`);
  console.log(`[update-kg] Total: ${graph.nodes.length} nodes, ${graph.edges.length} edges`);
  return graph;
}

/** 7. 重建层级 */
function rebuildLayers(graph) {
  const allFileLayers = new Map();

  for (const node of graph.nodes) {
    const nid = node.id;
    if (!nid || !nid.includes(':')) continue;
    const path = nid.slice(nid.indexOf(':') + 1);

    let bestLayer = null;
    let bestLen = 0;
    for (const rule of LAYER_RULES) {
      if (path.startsWith(rule.prefix) && rule.prefix.length > bestLen) {
        bestLayer = rule.name;
        bestLen = rule.prefix.length;
      }
    }

    if (bestLayer) {
      if (!allFileLayers.has(bestLayer)) allFileLayers.set(bestLayer, []);
      allFileLayers.get(bestLayer).push(nid);
    }
  }

  graph.layers = Array.from(allFileLayers.entries()).map(([name, nodeIds]) => ({
    id: name.toLowerCase().replace(/\//g, '-').replace(/ /g, '-'),
    name,
    description: `${name} layer`,
    nodeIds: [...new Set(nodeIds)],
  }));

  console.log(`[update-kg] Layers: ${graph.layers.length}`);
  for (const l of graph.layers) {
    console.log(`  ${l.name}: ${l.nodeIds.length} nodes`);
  }
}

/** 8. 保存（原子写入：临时文件 + rename） */
function save(graph, currentHead, analyzedFiles) {
  const nodeIDs = new Set(graph.nodes.map(n => n.id));
  const before = graph.edges.length;
  graph.edges = graph.edges.filter(e => nodeIDs.has(e.source) && nodeIDs.has(e.target));
  if (graph.edges.length !== before) {
    console.log(`[update-kg] Cleaned ${before - graph.edges.length} dangling edge(s)`);
  }

  const seen = new Set();
  const deduped = [];
  for (const n of graph.nodes) {
    if (!seen.has(n.id)) {
      seen.add(n.id);
      deduped.push(n);
    }
  }
  if (deduped.length !== graph.nodes.length) {
    console.log(`[update-kg] Removed ${graph.nodes.length - deduped.length} duplicate node(s)`);
    graph.nodes = deduped;
  }

  const now = new Date().toISOString().replace(/\.\d{3}Z$/, '.000Z');
  if (!graph.project) graph.project = {};
  graph.project.gitCommitHash = currentHead;
  graph.project.analyzedAt = now;
  graph.project.version = graph.project.version || '1.0.0';

  const tmpGraph = GRAPH_PATH + '.tmp';
  const tmpMeta = META_PATH + '.tmp';

  try {
    writeFileSync(tmpGraph, JSON.stringify(graph, null, 1), 'utf-8');
    renameSync(tmpGraph, GRAPH_PATH);
    console.log(`[update-kg] Written: ${GRAPH_PATH}`);

    const meta = {
      lastAnalyzedAt: now,
      gitCommitHash: currentHead,
      version: '1.0.0',
      analyzedFiles,
    };
    writeFileSync(tmpMeta, JSON.stringify(meta, null, 2), 'utf-8');
    renameSync(tmpMeta, META_PATH);
    console.log(`[update-kg] Written: ${META_PATH}`);
  } catch (err) {
    try { unlinkSync(tmpGraph); } catch {}
    try { unlinkSync(tmpMeta); } catch {}
    console.error(`[update-kg] Write failed: ${err.message}`);
    console.error('[update-kg] Original files preserved unchanged.');
    throw err;
  }
  console.log(`[update-kg] Updated to commit ${currentHead}`);
}

// ========================================================================
// 辅助
// ========================================================================

const EXT_TO_LANG = {
  '.go': 'go', '.ts': 'typescript', '.tsx': 'tsx', '.js': 'javascript', '.jsx': 'javascript',
  '.css': 'css', '.html': 'html', '.json': 'config', '.yaml': 'config', '.yml': 'config',
  '.toml': 'config', '.md': 'markdown', '.py': 'python', '.mjs': 'javascript', '.cjs': 'javascript',
};

function guessLanguage(filePath) {
  const ext = filePath.slice(filePath.lastIndexOf('.')).toLowerCase();
  return EXT_TO_LANG[ext] || 'unknown';
}

function guessCategory(filePath) {
  const l = guessLanguage(filePath);
  if (l === 'config') return 'config';
  if (l === 'markdown') return 'docs';
  return 'code';
}

// ========================================================================
// Main
// ========================================================================

async function main() {
  const isFull = process.argv.includes('--full');

  console.log('========================================');
  console.log(`  update-kg.mjs — Knowledge Graph Update (${isFull ? 'FULL' : 'incremental'})`);
  console.log('========================================\n');

  const { meta, graph } = readConfig();

  // 全量模式：从空图谱开始重建
  const graphToUse = graph || { nodes: [], edges: [], layers: [], project: { version: '1.0.0' } };
  if (!graph && isFull) {
    graphToUse.nodes = [];
    graphToUse.edges = [];
    graphToUse.layers = [];
    graphToUse.project = { version: '1.0.0' };
    console.log('[update-kg] Full mode: starting from empty graph.');
  }
  if (!graph && !isFull) {
    console.error('[update-kg] No existing knowledge graph found. Use --full to build from scratch.');
    process.exit(1);
  }

  const { changed, newFiles, deletedFiles, currentHead } = detectChanges(meta?.gitCommitHash, isFull);
  if (changed.length === 0) {
    process.exit(0);
  }

  // 应用 .understandignore 过滤
  const filteredChanged = await filterIgnoredFiles(changed, PROJECT_ROOT);
  const filteredNewFiles = await filterIgnoredFiles(newFiles, PROJECT_ROOT);

  // 全量模式：清除所有已有节点，从零重建
  const removeIDs = isFull
    ? new Set(graphToUse.nodes.map(n => n.id))
    : collectOldNodeIDs(graphToUse, changed, deletedFiles);

  const extractResults = await extractStructures(filteredChanged);
  const { newNodes, newEdges } = transformToGraphNodes(extractResults, filteredChanged);
  mergeIntoGraph(graphToUse, removeIDs, newNodes, newEdges);
  rebuildLayers(graphToUse);
  save(graphToUse, currentHead, graphToUse.nodes.length);

  console.log('\n[update-kg] Done.');
}

main().catch(err => {
  console.error('[update-kg] Fatal error:', err.message);
  process.exit(1);
});
