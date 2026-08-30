import assert from "node:assert/strict";
import { readdir, readFile } from "node:fs/promises";
import { basename, dirname, join, relative } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";
import ts from "typescript";
import { createAppStore } from "./appStore.ts";

const STORES_DIRECTORY = dirname(fileURLToPath(import.meta.url));

test("the shared factory exposes only state and actions", () => {
  const store = createAppStore(
    { count: 0 },
    ({ setState }) => ({
      increment: () => setState((state) => ({ count: state.count + 1 })),
    }),
  );

  assert.deepEqual(Object.keys(store.getState()).sort(), ["actions", "state"]);
  store.getState().actions.increment();
  assert.equal(store.getState().state.count, 1);
});

test("stores use an approved factory and keep models and config in their layers", async () => {
  const files = await typescriptSources(STORES_DIRECTORY);

  for (const file of files) {
    if (file.endsWith(".test.ts")) continue;
    const source = await readFile(file, "utf8");
    assertStoreLayerBoundaries(file, source);
    if (basename(file) !== "appStore.ts" && basename(file).endsWith("Store.ts")) {
      assert.match(
        source,
        /\b(?:createAppStore|createStore)(?:\s*<|\s*\()/,
        `${relative(STORES_DIRECTORY, file)} must use createAppStore or Zustand createStore`,
      );
    }
  }
});

function assertStoreLayerBoundaries(file: string, source: string): void {
  const module = ts.createSourceFile(file, source, ts.ScriptTarget.Latest);
  const path = relative(STORES_DIRECTORY, file);

  function visit(node: ts.Node): void {
    const line = module.getLineAndCharacterOfPosition(node.getStart(module)).line + 1;
    assert.ok(
      !ts.isInterfaceDeclaration(node)
        && !ts.isTypeAliasDeclaration(node)
        && !ts.isEnumDeclaration(node)
        && !ts.isTypeLiteralNode(node),
      `${path}:${line} must declare models and contracts in models/`,
    );
    ts.forEachChild(node, visit);
  }
  visit(module);

  for (const statement of module.statements) {
    if (!ts.isVariableStatement(statement)) continue;
    if (!(statement.declarationList.flags & ts.NodeFlags.Const)) continue;
    for (const declaration of statement.declarationList.declarations) {
      if (!ts.isIdentifier(declaration.name)) continue;
      assert.doesNotMatch(
        declaration.name.text,
        /^[A-Z][A-Z0-9_]*$/,
        `${path} must declare named fixed defaults and settings in config/`,
      );
    }
  }
}

async function typescriptSources(directory: string): Promise<string[]> {
  const entries = await readdir(directory, { withFileTypes: true });
  const files = await Promise.all(entries.map(async (entry) => {
    const path = join(directory, entry.name);
    if (entry.isDirectory()) return typescriptSources(path);
    return entry.isFile() && entry.name.endsWith(".ts") ? [path] : [];
  }));
  return files.flat().sort();
}
