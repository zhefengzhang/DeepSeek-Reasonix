import { invoke } from "@tauri-apps/api/core";
import type { TsDefinitionResult } from "../protocol";
import { useCallback, useEffect, useRef, useState } from "react";
import * as monaco from "monaco-editor";
import { I } from "../icons";
import type { Theme } from "../theme";
import { ConfirmDialog } from "./confirm-dialog";

// Vite worker imports for local monaco workers
import EditorWorker from "monaco-editor/esm/vs/editor/editor.worker.js?worker";
import TsWorker from "monaco-editor/esm/vs/language/typescript/ts.worker.js?worker";

self.MonacoEnvironment = {
  getWorker(_worker: unknown, label: string) {
    if (label === "typescript" || label === "javascript") {
      return new TsWorker();
    }
    return new EditorWorker();
  },
};

function extToLang(filePath: string): string {
  const name = filePath.split(/[\\/]/).pop() ?? "";
  const dot = name.lastIndexOf(".");
  if (dot < 0) return "plaintext";
  const ext = name.slice(dot + 1).toLowerCase();
  const map: Record<string, string> = {
    ts: "typescript",
    tsx: "typescript",
    js: "javascript",
    jsx: "javascript",
    json: "json",
    md: "markdown",
    css: "css",
    scss: "scss",
    html: "html",
    xml: "xml",
    yaml: "yaml",
    yml: "yaml",
    py: "python",
    rs: "rust",
    rb: "ruby",
    go: "go",
    java: "java",
    c: "c",
    cpp: "cpp",
    h: "c",
    hpp: "cpp",
    cs: "csharp",
    php: "php",
    sh: "shell",
    bash: "shell",
    zsh: "shell",
    sql: "sql",
    graphql: "graphql",
    gql: "graphql",
    vue: "html",
    svelte: "html",
    swift: "swift",
    kt: "kotlin",
    dart: "dart",
    lua: "lua",
    toml: "ini",
    ini: "ini",
    cfg: "ini",
    conf: "ini",
    env: "ini",
    dockerfile: "dockerfile",
    makefile: "makefile",
    tex: "latex",
    svg: "xml",
    txt: "plaintext",
    log: "plaintext",
  };
  return map[ext] ?? "plaintext";
}

export function FileContentViewer({
  openFiles,
  activeFile,
  theme,
  workspaceDir,
  onSetActiveFile,
  onOpenFile,
  onCloseFile,
}: {
  openFiles: string[];
  activeFile: string | null;
  theme: Theme;
  workspaceDir?: string;
  onSetActiveFile: (path: string) => void;
  onOpenFile?: (path: string) => void;
  onCloseFile: (path: string) => void;
}) {
  const editorRef = useRef<monaco.editor.IStandaloneCodeEditor | null>(null);
  const editorElRef = useRef<HTMLDivElement>(null);
  const activeFileRef = useRef<string | null>(null);

  // Cache: Map<path, { content: string; original: string; dirty: boolean }>
  const fileCache = useRef<
    Map<string, { content: string; original: string; dirty: boolean }>
  >(new Map());

  const [loadingFiles, setLoadingFiles] = useState<Set<string>>(new Set());
  const [closingFile, setClosingFile] = useState<{ path: string; name: string } | null>(null);
  const [contextMenu, setContextMenu] = useState<{ x: number; y: number; path: string } | null>(null);
  const [batchClosing, setBatchClosing] = useState<{ files: string[]; dirtyCount: number } | null>(null);
  const [copyToast, setCopyToast] = useState<string | null>(null);

  const flashCopyToast = useCallback((msg: string) => {
    setCopyToast(msg);
    setTimeout(() => setCopyToast(null), 1500);
  }, []);

  // Close context menu on click outside
  useEffect(() => {
    if (!contextMenu) return;
    const close = () => setContextMenu(null);
    // Delay to avoid the same click that opened it
    requestAnimationFrame(() => window.addEventListener("click", close, { once: true }));
    return () => window.removeEventListener("click", close);
  }, [contextMenu]);

  // Register each opened file as a model with a proper file:// URI so
  // the TypeScript language service can resolve relative imports between
  // files. Without this every `import` from a sibling file shows "Cannot
  // find module".
  function fileUri(path: string): monaco.Uri {
    // Uri.file handles both Windows drive letters (D:/…) and POSIX paths.
    return monaco.Uri.file(path.replace(/\\/g, "/"));
  }

  function getOrCreateModel(
    path: string,
    content: string,
    language: string,
  ): monaco.editor.ITextModel {
    const uri = fileUri(path);
    const existing = monaco.editor.getModel(uri);
    if (existing) {
      if (existing.getValue() !== content) existing.setValue(content);
      return existing;
    }
    return monaco.editor.createModel(content, language, uri);
  }

  // Compute a relative path for use in @mentions / file references.
  function relPath(abs: string): string {
    if (!workspaceDir) return abs;
    const ws = workspaceDir.replace(/\\/g, "/").replace(/\/+$/, "");
    const norm = abs.replace(/\\/g, "/");
    if (norm.toLowerCase().startsWith(ws.toLowerCase() + "/")) {
      return norm.slice(ws.length + 1);
    }
    return abs;
  }

  // Copy selected code with file:line reference (Cursor-style).
  function copySelectionRef() {
    const editor = editorRef.current;
    const filePath = activeFileRef.current;
    if (!editor || !filePath) return;
    const selection = editor.getSelection();
    if (!selection || selection.isEmpty()) return;
    const model = editor.getModel();
    if (!model) return;
    const startLine = selection.startLineNumber;
    const endLine = selection.endLineNumber;
    const range = startLine === endLine ? `${startLine}` : `${startLine}-${endLine}`;
    const ref = `@${relPath(filePath)}:${range}`;
    const selectedText = model.getValueInRange(selection);
    // Only include code block if selection spans multiple lines or is
    // non-trivial (longer than 80 chars or contains newlines).
    const useBlock = selectedText.includes("\n") || selectedText.length > 80;
    const payload = useBlock
      ? `${ref}\n\`\`\`${extToLang(filePath)}\n${selectedText}\n\`\`\``
      : `${ref} — \`${selectedText}\``;
    void navigator.clipboard.writeText(payload);
    flashCopyToast("Copied with file reference");
  }

  // When a file is opened, scan its import statements and pre-register
  // the imported modules as Monaco models so go-to-definition and type
  // resolution work across project files.
  const preloadingRef = useRef<Set<string>>(new Set());
  // Derive the current file's directory (normalized to forward slashes)
  function fileDir(abs: string): string {
    return abs.replace(/\\/g, "/").replace(/\/[^/]*$/, "");
  }

  async function preloadImports(absPath: string, content: string) {
    if (!workspaceDir) return;
    const dir = fileDir(absPath);
    // Match `from "./foo"` / `from "../bar"` (both single and double quotes)
    const IMPORT_RE = /from\s+['"](\..*?)['"]/g;
    const jobs: string[] = [];
    let m: RegExpExecArray | null;
    while ((m = IMPORT_RE.exec(content)) !== null) {
      const importPath = m[1]!;
      // Resolve to the actual .ts file on disk.  The definition provider
      // returns Location at this URI; go-to-definition navigates here.
      const tsPath = resolveImportPathWin(dir, importPath.replace(/\.js$/, ".ts"))
        ?? resolveImportPathWin(dir, importPath);
      if (tsPath && !preloadingRef.current.has(tsPath)) {
        jobs.push(tsPath);
      }
    }
    if (jobs.length === 0) return;
    const results = await Promise.allSettled(
      jobs.map(async (p) => {
        preloadingRef.current.add(p);
        const text = await invoke<string>("read_text_file", { path: p });
        const lang = extToLang(p);
        getOrCreateModel(p, text, lang);
      }),
    );
    for (let i = 0; i < results.length; i++) {
      if (results[i]?.status === "rejected") {
        preloadingRef.current.delete(jobs[i]!);
      }
    }
  }

  // Simple path resolution for Windows/Linux without pull-up of node:path
  function resolveImportPathWin(dir: string, relative: string): string | null {
    // Handle ".." and "." in the relative path
    const parts = relative.split("/");
    const dirParts = dir.split("/");
    const result: string[] = [];
    for (const p of parts) {
      if (p === "." || p === "") continue;
      if (p === "..") {
        if (result.length > 0) result.pop();
        else if (dirParts.length > 0) dirParts.pop();
      } else {
        result.push(p);
      }
    }
    const base = dirParts.join("/");
    const resolved = base ? `${base}/${result.join("/")}` : result.join("/");
    // Try each extension — return the first one that already has a model
    // (meaning it was loaded before), otherwise return the .ts variant.
    const exts = ["", ".ts", ".tsx", "/index.ts", "/index.tsx"];
    for (const ext of exts) {
      const candidate = resolved + ext;
      if (monaco.editor.getModel(fileUri(candidate))) {
        return candidate;
      }
    }
    // Don't double-append extension
    if (/\.(ts|tsx|js|jsx)$/.test(resolved)) return resolved;
    return resolved + ".ts";
  }

  // Create editor ONCE on mount
  useEffect(() => {
    const el = editorElRef.current;
    if (!el) return;

    // Configure TS language service so imports between opened files work.
    // monaco 0.44+ moved the API from monaco.languages.typescript → monaco.typescript.
    const ts = monaco.typescript;
    ts.typescriptDefaults.setCompilerOptions({
      target: ts.ScriptTarget.ES2020,
      module: ts.ModuleKind.ESNext,
      // Monaco's embedded TS defaults to NodeJs resolution, which
      // doesn't remap .js → .ts imports.  We suppress the resulting
      // TS2307 to avoid false positives; remaining type checking and
      // navigation still work correctly within each file.
      moduleResolution: ts.ModuleResolutionKind.NodeJs,
      allowNonTsExtensions: true,
      allowJs: true,
      checkJs: false,
      jsx: ts.JsxEmit.ReactJSX,
      noEmit: true,
      esModuleInterop: true,
      isolatedModules: true,
      strict: true,
    });
    ts.typescriptDefaults.setEagerModelSync(true);

    // Suppress "Cannot find module" (TS2307) — the language service has no
    // access to the actual file system, so import resolution fails for any
    // file not currently open as a model.  Cross-file resolution between
    // OPEN models still works because we register each one with a file://
    // URI via getOrCreateModel().
    ts.typescriptDefaults.setDiagnosticsOptions({
      diagnosticCodesToIgnore: [2307],
    });

    const editor = monaco.editor.create(el, {
      value: "",
      language: "plaintext",
      theme: theme === "dark" ? "vs-dark" : "vs",
      fontSize: 13,
      fontFamily: "Geist Mono, 'Courier New', monospace",
      lineNumbers: "on",
      minimap: { enabled: true },
      scrollBeyondLastLine: false,
      wordWrap: "off",
      tabSize: 2,
      renderWhitespace: "selection",
      bracketPairColorization: { enabled: true },
      autoClosingBrackets: "always",
      autoIndent: "full",
      formatOnPaste: true,
      cursorBlinking: "smooth",
      smoothScrolling: true,
      padding: { top: 12 },
      readOnly: false,
    });

    editorRef.current = editor;

    // Track dirty state on content change
    editor.onDidChangeModelContent(() => {
      const activePath = activeFileRef.current;
      if (!activePath) return;
      const entry = fileCache.current.get(activePath);
      if (!entry) return;
      entry.content = editor.getValue();
      entry.dirty = entry.content !== entry.original;
    });

    // Ctrl+S — auto-save
    editor.addCommand(monaco.KeyMod.CtrlCmd | monaco.KeyCode.KeyS, async () => {
      const activePath = activeFileRef.current;
      if (!activePath) return;
      const entry = fileCache.current.get(activePath);
      if (!entry || !entry.dirty) return;
      try {
        await invoke("write_text_file", { path: activePath, content: entry.content });
        entry.original = entry.content;
        entry.dirty = false;
      } catch (err) {
        console.error("save failed", err);
      }
    });

    // Ctrl+Shift+C — copy selection with file:line reference (Cursor-style)
    editor.addAction({
      id: "copy-selection-ref",
      label: "Copy with Reference",
      keybindings: [monaco.KeyMod.CtrlCmd | monaco.KeyMod.Shift | monaco.KeyCode.KeyC],
      contextMenuGroupId: "9_cutcopypaste",
      contextMenuOrder: 2.1,
      precondition: "editorHasSelection",
      run: () => copySelectionRef(),
    });

    // F12 / Ctrl+Click — go-to-definition via TypeScript language server.
    // Uses the project's real tsconfig.json for precise symbol resolution.
    const triggerDefinition = async () => {
      const filePath = activeFileRef.current;
      const pos = editor.getPosition();
      if (!filePath || !pos || !workspaceDir || !onOpenFile) {
        console.log("[go-to-def] missing prereq", { filePath, pos, workspaceDir, onOpenFile });
        return;
      }
      console.log("[go-to-def] calling ts_definition", { filePath, line: pos.lineNumber, col: pos.column });
      try {
        const result = await invoke<TsDefinitionResult>("ts_definition", {
          root: workspaceDir,
          file: filePath,
          line: pos.lineNumber,
          column: pos.column,
        });
        console.log("[go-to-def] result", result);
        if (result && result.file) {
          // ts-lookup now returns an absolute path — use it directly.
          console.log("[go-to-def] opening", result.file);
          onOpenFile(result.file);
          // Scroll to the definition line after the model loads
          setTimeout(() => {
            const e = editorRef.current;
            if (e && activeFileRef.current === result.file) {
              e.revealLineInCenter(result.line);
              e.setPosition({ lineNumber: result.line, column: result.column });
            }
          }, 300);
          return;
        }
      } catch (err) {
        console.log("[go-to-def] ts_definition failed", err);
        // ts_definition not available or no result — fall through
      }
      // Fallback: try to resolve import paths directly
      const model = editor.getModel();
      if (!model) return;
      const line = model.getLineContent(pos.lineNumber);
      const m =
        line.match(/from\s+['"]([^'"]+)['"]/) ??
        line.match(/import\s+['"]([^'"]+)['"]/);
      if (!m || !m[1]?.startsWith(".")) return;
      const importPath = m[1];
      const dir = fileDir(filePath);
      const tsPath =
        resolveImportPathWin(dir, importPath.replace(/\.js$/, ".ts")) ??
        resolveImportPathWin(dir, importPath);
      if (tsPath) onOpenFile(tsPath);
    };

    // F12 via keyboard — use browserEvent.key (not .keyCode which is 0
    // in modern Chromium for function keys).
    editor.onKeyDown((e) => {
      const key = e.browserEvent?.key ?? String.fromCharCode(e.keyCode);
      if (key === "F12" && !e.ctrlKey && !e.altKey && !e.metaKey) {
        e.preventDefault();
        e.stopPropagation();
        console.log("[go-to-def] F12 pressed, triggering…");
        void triggerDefinition();
      }
    });

    // Ctrl+Click via mouse (Monaco fires go-to-definition on Ctrl+Click)
    editor.onMouseDown((e) => {
      if (e.event.ctrlKey || e.event.metaKey) {
        // Let Monaco handle the selection, then intercept the definition
        // request that follows. We use a microtask to run after Monaco's
        // click handler.
        queueMicrotask(() => {
          void triggerDefinition();
        });
      }
    });

    requestAnimationFrame(() => editor.layout());

    return () => {
      editor.dispose();
      editorRef.current = null;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Switch active file: save current, load new
  useEffect(() => {
    const editor = editorRef.current;
    if (!editor) return;

    // Persist current editor content to cache
    const prevPath = activeFileRef.current;
    if (prevPath) {
      const prevEntry = fileCache.current.get(prevPath);
      if (prevEntry) {
        prevEntry.content = editor.getValue();
        prevEntry.dirty = prevEntry.content !== prevEntry.original;
      }
    }

    activeFileRef.current = activeFile;

    if (!activeFile) return;

    const newLang = extToLang(activeFile);

    const cached = fileCache.current.get(activeFile);
    const loadAndSwitch = async () => {
      // Load file content (from cache or disk) and preload imported models
      // BEFORE calling setModel, so the TS worker finds them when it
      // resolves imports on the initial pass.
      let text: string;
      if (cached) {
        text = cached.content;
      } else {
        setLoadingFiles((prev) => new Set(prev).add(activeFile));
        try {
          text = await invoke<string>("read_text_file", { path: activeFile });
          fileCache.current.set(activeFile, { content: text, original: text, dirty: false });
        } catch (err) {
          console.error("load failed", err);
          return;
        } finally {
          setLoadingFiles((prev) => {
            const next = new Set(prev);
            next.delete(activeFile);
            return next;
          });
        }
      }
      // Preload imports first — create models so TS worker sees them
      await preloadImports(activeFile, text);
      // Now set the main model — TS worker resolves imports successfully
      if (activeFileRef.current === activeFile) {
        editor.setModel(getOrCreateModel(activeFile, text, newLang));
      }
    };
    void loadAndSwitch();

    requestAnimationFrame(() => editor.layout());
  }, [activeFile]);

  // Follow theme
  useEffect(() => {
    monaco.editor.setTheme(theme === "dark" ? "vs-dark" : "vs");
  }, [theme]);

  // Resize observer
  useEffect(() => {
    const container = document.querySelector(".fcv-body");
    if (!container) return;
    const ro = new ResizeObserver(() => {
      editorRef.current?.layout();
    });
    ro.observe(container);
    return () => ro.disconnect();
  }, []);

  const handleTabContextMenu = useCallback((path: string, e: React.MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();
    setContextMenu({ x: e.clientX, y: e.clientY, path });
  }, []);

  const doCloseFiles = useCallback((files: string[]) => {
    files.forEach((f) => {
      fileCache.current.delete(f);
      const uri = fileUri(f);
      monaco.editor.getModel(uri)?.dispose();
    });
    setLoadingFiles((prev) => {
      const next = new Set(prev);
      files.forEach((f) => next.delete(f));
      return next;
    });
    files.forEach((f) => onCloseFile(f));
  }, [onCloseFile]);

  const doCloseFile = useCallback((path: string) => {
    doCloseFiles([path]);
  }, [doCloseFiles]);

  const promptCloseFile = useCallback((path: string) => {
    const entry = fileCache.current.get(path);
    if (entry?.dirty) {
      setClosingFile({ path, name: path.split(/[\\/]/).pop() ?? path });
      return;
    }
    doCloseFile(path);
  }, []);

  const saveFile = useCallback(async (path: string) => {
    const entry = fileCache.current.get(path);
    if (!entry?.dirty) return;
    const editor = editorRef.current;
    if (editor) entry.content = editor.getValue();
    await invoke("write_text_file", { path, content: entry.content });
    entry.original = entry.content;
    entry.dirty = false;
  }, []);

  // Context menu actions
  const handleCtxClose = useCallback(() => {
    const p = contextMenu?.path;
    setContextMenu(null);
    if (!p) return;
    promptCloseFile(p);
  }, [contextMenu, promptCloseFile]);

  const handleCtxCloseOthers = useCallback(() => {
    const p = contextMenu?.path;
    setContextMenu(null);
    if (!p) return;
    const others = openFiles.filter((f) => f !== p);
    const dirty = others.filter((f) => fileCache.current.get(f)?.dirty);
    if (dirty.length > 0) {
      setBatchClosing({ files: others, dirtyCount: dirty.length });
    } else {
      doCloseFiles(others);
    }
  }, [contextMenu, openFiles, doCloseFiles]);

  const handleCtxCloseRight = useCallback(() => {
    const p = contextMenu?.path;
    setContextMenu(null);
    if (!p) return;
    const idx = openFiles.indexOf(p);
    if (idx < 0) return;
    const right = openFiles.slice(idx + 1);
    if (right.length === 0) return;
    const dirty = right.filter((f) => fileCache.current.get(f)?.dirty);
    if (dirty.length > 0) {
      setBatchClosing({ files: right, dirtyCount: dirty.length });
    } else {
      doCloseFiles(right);
    }
  }, [contextMenu, openFiles, doCloseFiles]);

  const handleCtxCloseSaved = useCallback(() => {
    const p = contextMenu?.path;
    setContextMenu(null);
    const saved = openFiles.filter((f) => {
      const entry = fileCache.current.get(f);
      return f !== p && entry && !entry.dirty;
    });
    doCloseFiles(saved);
  }, [contextMenu, openFiles, doCloseFiles]);

  const handleCtxCloseAll = useCallback(() => {
    setContextMenu(null);
    const dirty = openFiles.filter((f) => fileCache.current.get(f)?.dirty);
    if (dirty.length > 0) {
      setBatchClosing({ files: [...openFiles], dirtyCount: dirty.length });
    } else {
      doCloseFiles([...openFiles]);
    }
  }, [openFiles, doCloseFiles]);

  // Single-file confirm handlers
  const handleConfirmSave = useCallback(async () => {
    const cf = closingFile;
    if (!cf) return;
    setClosingFile(null);
    const editor = editorRef.current;
    const entry = fileCache.current.get(cf.path);
    if (editor && entry) {
      entry.content = editor.getValue();
      try {
        await invoke("write_text_file", { path: cf.path, content: entry.content });
      } catch (err) {
        console.error("save failed", err);
      }
    }
    doCloseFile(cf.path);
  }, [closingFile, doCloseFile]);

  const handleConfirmDiscard = useCallback(() => {
    const cf = closingFile;
    if (!cf) return;
    setClosingFile(null);
    doCloseFile(cf.path);
  }, [closingFile, doCloseFile]);

  const handleConfirmCancel = useCallback(() => {
    setClosingFile(null);
  }, []);
  const handleBatchSave = useCallback(async () => {
    const b = batchClosing;
    if (!b) return;
    setBatchClosing(null);
    for (const f of b.files) {
      try {
        await saveFile(f);
      } catch (err) {
        console.error("save failed", err);
      }
    }
    doCloseFiles(b.files);
  }, [batchClosing, saveFile, doCloseFiles]);

  const handleBatchDiscard = useCallback(() => {
    const b = batchClosing;
    if (!b) return;
    setBatchClosing(null);
    doCloseFiles(b.files);
  }, [batchClosing, doCloseFiles]);

  const handleBatchCancel = useCallback(() => {
    setBatchClosing(null);
  }, []);

  const isLoading = activeFile ? loadingFiles.has(activeFile) : false;
  const showContextMenu = contextMenu !== null;

  const ctxItems = [
    { label: "关闭", fn: handleCtxClose },
    { label: "关闭其他", fn: handleCtxCloseOthers },
    { label: "关闭右侧标签页", fn: handleCtxCloseRight },
    { label: "关闭已保存", fn: handleCtxCloseSaved },
    { label: "全部关闭", fn: handleCtxCloseAll },
  ];

  return (
    <div className="file-content-viewer">
      {/* Tab bar */}
      <div className="editor-tabs">
        {openFiles.map((path) => {
          const name = path.split(/[\\/]/).pop() ?? path;
          const entry = fileCache.current.get(path);
          const isDirty = entry?.dirty ?? false;
          return (
            <div
              key={path}
              className={`editor-tab${path === activeFile ? " active" : ""}`}
              onClick={() => onSetActiveFile(path)}
              onContextMenu={(e) => handleTabContextMenu(path, e)}
            >
              <span className="editor-tab-name">{name}</span>
              {isDirty ? <span className="editor-tab-dirty"> ●</span> : null}
              <button
                type="button"
                className="editor-tab-close"
                onClick={(e) => {
                  e.stopPropagation();
                  promptCloseFile(path);
                }}
              >
                <I.x size={12} />
              </button>
            </div>
          );
        })}
      </div>

      {/* Context menu */}
      {showContextMenu ? (
        <div
          className="ctx-menu"
          style={{ left: contextMenu.x, top: contextMenu.y }}
        >
          {ctxItems.map((item) => (
            <div key={item.label} className="ctx-menu-item" onClick={item.fn}>
              {item.label}
            </div>
          ))}
        </div>
      ) : null}

      {/* Monaco editor container */}
      <div className="fcv-body">
        <div ref={editorElRef} className="fcv-monaco-editor" />
        {isLoading ? (
          <div className="fcv-loading">
            <span className="ft-loading-spinner" />
            <span>加载中...</span>
          </div>
        ) : null}
        {closingFile ? (
          <ConfirmDialog
            open
            title="未保存的更改"
            message={<>是否在关闭 <strong>{closingFile.name}</strong> 前保存更改？</>}
            saveLabel="保存"
            discardLabel="不保存"
            cancelLabel="取消"
            onSave={handleConfirmSave}
            onDiscard={handleConfirmDiscard}
            onCancel={handleConfirmCancel}
          />
        ) : null}
        {batchClosing ? (
          <ConfirmDialog
            open
            title="未保存的更改"
            message={`有 ${batchClosing.dirtyCount} 个文件有未保存的更改。`}
            saveLabel="保存并关闭"
            discardLabel="不保存直接关闭"
            cancelLabel="取消"
            onSave={handleBatchSave}
            onDiscard={handleBatchDiscard}
            onCancel={handleBatchCancel}
          />
        ) : null}
      </div>
      {copyToast ? <div className="fcv-copy-toast">{copyToast}</div> : null}
    </div>
  );
}
