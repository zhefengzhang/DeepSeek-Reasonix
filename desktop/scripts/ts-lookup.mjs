#!/usr/bin/env node
/**
 * Persistent TypeScript go-to-definition server.
 *
 * Reads JSON lines from stdin, writes JSON lines to stdout.
 * Each line: { "root": "...", "file": "...", "line": N, "column": N }
 * Response:   { "file": "...", "line": N, "column": N } or null
 *
 * Exit on EOF or {"exit":true}.
 */

import { createRequire } from "node:module";
import { createInterface } from "node:readline";
import { dirname, join, relative } from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = dirname(fileURLToPath(import.meta.url));
const repoRoot = join(__dirname, "..", "..");
const requireTs = createRequire(join(repoRoot, "package.json"));
const ts = requireTs("typescript");

/** Per-project language service cache. Keyed by root dir. */
const services = new Map();

function getService(rootDir) {
  let entry = services.get(rootDir);
  if (entry) return entry;

  const configPath = join(rootDir, "tsconfig.json");
  const { config: parsed } = ts.readConfigFile(configPath, ts.sys.readFile);
  const result = ts.parseJsonConfigFileContent(parsed, ts.sys, rootDir);
  const fileNames = [...result.fileNames];
  const options = result.options;

  const host = {
    getScriptFileNames: () => fileNames,
    getScriptVersion: () => "0",
    getScriptSnapshot: (fileName) => {
      try {
        const text = ts.sys.readFile(fileName);
        return text !== undefined ? ts.ScriptSnapshot.fromString(text) : undefined;
      } catch {
        return undefined;
      }
    },
    getCurrentDirectory: () => rootDir,
    getCompilationSettings: () => options,
    getDefaultLibFileName: (opts) => ts.getDefaultLibFilePath(opts),
    fileExists: ts.sys.fileExists,
    readFile: ts.sys.readFile,
    readDirectory: ts.sys.readDirectory,
    directoryExists: ts.sys.directoryExists,
    getDirectories: ts.sys.getDirectories,
  };

  const service = ts.createLanguageService(host, ts.createDocumentRegistry());
  entry = { service, fileNames, rootDir, options };
  services.set(rootDir, entry);
  return entry;
}

function lookup({ root, file, line, column }) {
  const entry = getService(root);

  // Add the target file if it's not already in the project
  if (!entry.fileNames.includes(file)) {
    entry.fileNames.push(file);
  }

  const program = entry.service.getProgram();
  if (!program) return null;

  const sourceFile = program.getSourceFile(file);
  if (!sourceFile) return null;

  const pos = ts.getPositionOfLineAndCharacter(sourceFile, line - 1, column - 1);

  let defs = entry.service.getDefinitionAtPosition(file, pos);
  if (!defs || defs.length === 0) {
    const span = entry.service.getDefinitionAndBoundSpan(file, pos);
    if (span?.definitions?.length) defs = span.definitions;
    else return null;
  }

  const def = defs[0];
  const defFile = program.getSourceFile(def.fileName);
  const defPos = defFile
    ? ts.getLineAndCharacterOfPosition(defFile, def.textSpan.start)
    : { line: 0, character: 0 };

  return {
    file: def.fileName.replace(/\\/g, "/"),
    line: defPos.line + 1,
    column: defPos.character + 1,
  };
}

// --- main loop ---

const rl = createInterface({ input: process.stdin, output: process.stdout, terminal: false });

rl.on("line", (raw) => {
  raw = raw.trim();
  if (!raw || raw === '{"exit":true}') {
    rl.close();
    return;
  }
  try {
    const req = JSON.parse(raw);
    const result = lookup(req);
    process.stdout.write(JSON.stringify(result ?? { error: "no definition found" }) + "\n");
  } catch (e) {
    process.stdout.write(JSON.stringify({ error: e.message }) + "\n");
  }
});

rl.on("close", () => process.exit(0));
