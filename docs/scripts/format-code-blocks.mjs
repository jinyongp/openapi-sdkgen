import { readFile, readdir, writeFile } from "node:fs/promises";
import { resolve, relative } from "node:path";
import { spawnSync } from "node:child_process";
import { format } from "prettier";

const root = resolve(import.meta.dirname, "../..");
const parsers = new Map([
  ["ts", "typescript"],
  ["typescript", "typescript"],
  ["tsx", "typescript"],
  ["js", "babel"],
  ["javascript", "babel"],
  ["jsx", "babel"],
  ["json", "json"],
  ["jsonc", "jsonc"],
  ["yaml", "yaml"],
  ["yml", "yaml"],
]);

// Replace fence contents only: prose, fence metadata and output examples stay intact.
export async function formatCodeBlocks(source, filename = "Markdown") {
  const opening = /^( {0,3})(`{3,}|~{3,})([^\n]*)\n/gm;
  const edits = [];
  let blocks = 0;
  for (let match; (match = opening.exec(source));) {
    const [, indent, fence, info] = match;
    const closing = new RegExp(
      `^ {0,3}${fence[0]}{${fence.length},}[ \\t]*(?:\\r?\\n|$)`,
      "gm",
    );
    closing.lastIndex = opening.lastIndex;
    const end = closing.exec(source);
    if (!end) break;
    const start = opening.lastIndex;
    opening.lastIndex = closing.lastIndex;
    const language = info.trim().split(/[\s{]/)[0];
    const parser = parsers.get(language);
    if (!parser && language !== "go") continue;
    const original = source.slice(start, end.index);
    const code = original.replace(new RegExp(`^${indent}`, "gm"), "");
    const line = source.slice(0, start).split("\n").length;
    let formatted;
    try {
      if (language === "go") {
        const result = spawnSync("gofmt", [], {
          input: code,
          encoding: "utf8",
        });
        if (result.error || result.status !== 0)
          throw new Error(result.error?.message ?? result.stderr.trim());
        formatted = result.stdout;
      } else {
        formatted = await format(code, { parser });
      }
    } catch (error) {
      throw new Error(`${filename}:${line} (${language}): ${error.message}`, {
        cause: error,
      });
    }
    if (indent) formatted = formatted.replace(/^(?=.)/gm, indent);
    blocks++;
    if (original !== formatted)
      edits.push({ start, end: end.index, formatted, line, language });
  }
  let output = source;
  for (const edit of edits.toReversed())
    output =
      output.slice(0, edit.start) + edit.formatted + output.slice(edit.end);
  return {
    source: output,
    blocks,
    changes: edits.map(({ line, language }) => ({ line, language })),
  };
}

async function markdownFiles(directory) {
  const files = [];
  for (const entry of await readdir(directory, { withFileTypes: true })) {
    if (entry.name.startsWith(".") || entry.name === "node_modules") continue;
    const path = resolve(directory, entry.name);
    if (entry.isDirectory()) files.push(...(await markdownFiles(path)));
    else if (entry.isFile() && entry.name.endsWith(".md")) files.push(path);
  }
  return files.sort();
}

async function main() {
  const args = process.argv.slice(2);
  if (args.length !== 1 || !["--check", "--write"].includes(args[0]))
    throw new Error("Usage: format-code-blocks.mjs --check|--write");
  const write = args[0] === "--write";
  const files = [
    resolve(root, "README.md"),
    ...(await markdownFiles(resolve(root, "docs"))),
  ];
  let blocks = 0;
  let changed = 0;
  for (const path of files) {
    const name = relative(root, path);
    const result = await formatCodeBlocks(await readFile(path, "utf8"), name);
    blocks += result.blocks;
    if (!result.changes.length) continue;
    changed++;
    if (write) await writeFile(path, result.source);
    else
      for (const change of result.changes)
        console.error(
          `${name}:${change.line}: unformatted ${change.language} code block`,
        );
  }
  if (changed && !write) {
    console.error(
      `Run devtools run format:docs to format ${changed} Markdown files.`,
    );
    process.exitCode = 1;
  } else
    console.log(
      `ok documentation code format: ${blocks} blocks, ${write ? `${changed} files updated` : "no changes needed"}`,
    );
}

if (process.argv[1] && resolve(process.argv[1]) === import.meta.filename) {
  main().catch((error) => {
    console.error(error.message);
    process.exitCode = 1;
  });
}
