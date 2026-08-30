import assert from "node:assert/strict";
import { readdir, readFile } from "node:fs/promises";
import { basename, dirname, join, relative } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";
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

test("domain stores cannot bypass the shared factory", async () => {
  const files = await typescriptSources(STORES_DIRECTORY);
  const zustandImports: string[] = [];

  for (const file of files) {
    if (file.endsWith(".test.ts")) continue;
    const source = await readFile(file, "utf8");
    if (/["']zustand(?:\/[^"']*)?["']/.test(source)) {
      zustandImports.push(relative(STORES_DIRECTORY, file));
    }
    if (basename(file) !== "appStore.ts" && basename(file).endsWith("Store.ts")) {
      assert.match(
        source,
        /\bcreateAppStore(?:\s*<|\s*\()/,
        `${relative(STORES_DIRECTORY, file)} must use createAppStore`,
      );
    }
  }

  assert.deepEqual(zustandImports, []);
});

async function typescriptSources(directory: string): Promise<string[]> {
  const entries = await readdir(directory, { withFileTypes: true });
  const files = await Promise.all(entries.map(async (entry) => {
    const path = join(directory, entry.name);
    if (entry.isDirectory()) return typescriptSources(path);
    return entry.isFile() && entry.name.endsWith(".ts") ? [path] : [];
  }));
  return files.flat().sort();
}
