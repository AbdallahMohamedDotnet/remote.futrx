import assert from "node:assert/strict";
import { readdir, readFile } from "node:fs/promises";
import { join, relative } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const SOURCE_DIRECTORY = fileURLToPath(new URL("../../", import.meta.url));

test("runtime code never loads Zustand's React binding", async () => {
  const files = await typescriptSources(SOURCE_DIRECTORY);

  for (const file of files) {
    if (file.endsWith(".test.ts") || file.endsWith(".test.tsx")) continue;
    const source = await readFile(file, "utf8");
    assert.doesNotMatch(
      source,
      /(?:from\s+|import\s*\()["']zustand["']/,
      `${relative(SOURCE_DIRECTORY, file)} must use the Preact-native adapter`,
    );
  }
});

async function typescriptSources(directory: string): Promise<string[]> {
  const entries = await readdir(directory, { withFileTypes: true });
  const files = await Promise.all(entries.map(async (entry) => {
    const path = join(directory, entry.name);
    if (entry.isDirectory()) return typescriptSources(path);
    return entry.isFile() && /\.tsx?$/.test(entry.name) ? [path] : [];
  }));
  return files.flat().sort();
}
