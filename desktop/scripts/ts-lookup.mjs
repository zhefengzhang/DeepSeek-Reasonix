#!/usr/bin/env node
/**
 * TypeScript go-to-definition lookup.
 *
 * Usage: node ts-lookup.mjs <rootDir> <filePath> <line> <column>
 * Output (stdout): { "file": "...", "line": N, "column": N }
 * Errors (stderr):  { "error": "..." }
 */

import { createRequire } from "node:module";
import { dirname, join, relative } from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = dirname(fileURLToPath(import.meta.url));
const repoRoot = join(__dirname, "..", "..");

const requireTs = createRequire(join(repoRoot, "package.json"));
const ts = requireTs("typescript");

function fail(msg) {
  process.stderr.write(JSON.stringify({ error: msg }) + "\n");
  process.exit(1);
}

const args = process.argv.slice(2);
if (args.length < 4) fail("usage: ts-lookup.mjs <rootDir> <filePath> <line> <column>");

const [rootDir, filePath, lineStr, columnStr] = args;
const line = Number(lineStr);
const column = Number(columnStr);
if (Number.isNaN(line) || Number.isNaN(column)) fail("invalid line/column");

// Read tsconfig.json
const configPath = join(rootDir, "tsconfig.json");
let options, fileNames;
try {
  const { config: parsed } = ts.readConfigFile(configPath, ts.sys.readFile);
  const result = ts.parseJsonConfigFileContent(parsed, ts.sys, rootDir);
  options = result.options;
  fileNames = result.fileNames;
} catch (e) {
  fail(`failed to read tsconfig: ${e.message}`);
}

// Include the target file even if it's excluded by tsconfig
if (!fileNames.includes(filePath)) {
  fileNames = [...fileNames, filePath];
}

// Build a language service
const files = new Map();
for (const fn of fileNames) {
  try { files.set(fn, ts.sys.readFile(fn)); } catch { /* skip */ }
}

const host = {
  getScriptFileNames: () => fileNames,
  getScriptVersion: () => "0",
  getScriptSnapshot: (fileName) => {
    let text = files.get(fileName);
    if (text === undefined) {
      try { text = ts.sys.readFile(fileName); } catch { return undefined; }
      if (text !== undefined) files.set(fileName, text);
    }
    return text !== undefined ? ts.ScriptSnapshot.fromString(text) : undefined;
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

// Open the target file to warm the service
const sourceFile = service.getProgram()?.getSourceFile(filePath);
if (!sourceFile) fail(`file not in program: ${filePath}`);

const pos = ts.getPositionOfLineAndCharacter(sourceFile, line - 1, column - 1);

// go-to-definition
let definitionResult = service.getDefinitionAtPosition(filePath, pos);
if (!definitionResult || definitionResult.length === 0) {
  // fallback: getDefinitionAndBoundSpan
  const span = service.getDefinitionAndBoundSpan(filePath, pos);
  if (span && span.definitions && span.definitions.length > 0) {
    definitionResult = span.definitions;
  } else {
    fail("no definition found");
  }
}

const def = definitionResult[0];
// Return the absolute path — the frontend opens files by absolute path.
const defFile = service.getProgram()?.getSourceFile(def.fileName);
const defPos = defFile
  ? ts.getLineAndCharacterOfPosition(defFile, def.textSpan.start)
  : { line: 0, character: 0 };

process.stdout.write(
  JSON.stringify({
    file: def.fileName.replace(/\\/g, "/"),
    line: defPos.line + 1,
    column: defPos.character + 1,
  }) + "\n",
);
