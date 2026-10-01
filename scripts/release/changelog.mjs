import { readFileSync, writeFileSync } from "node:fs";
import { pathToFileURL } from "node:url";

export function prepareChangelog(text, tag, date) {
  if (!/^v\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?$/.test(tag)) {
    throw new Error("Invalid changelog version");
  }
  if (!/^\d{4}-\d{2}-\d{2}$/.test(date)) throw new Error("Invalid changelog date");
  const sections = [...text.matchAll(/^## (.+)$/gm)];
  const unreleased = sections.filter((match) => match[1] === "Unreleased");
  if (unreleased.length !== 1 || sections[0] !== unreleased[0]) {
    throw new Error("CHANGELOG.md must start with exactly one ## Unreleased section");
  }
  const start = unreleased[0].index;
  const end = sections[1]?.index ?? text.length;
  const notes = text.slice(start + unreleased[0][0].length, end).trim();
  const existing = sections.filter((match) => match[1].split(" — ")[0] === tag);
  if (existing.length) {
    // A failed check/push leaves the preparation commit available for retry.
    if (existing.length === 1 && existing[0] === sections[1] && !notes &&
        new RegExp(`^${tag.replaceAll(".", "\\.")} — \\d{4}-\\d{2}-\\d{2}$`).test(existing[0][1]) &&
        text.slice(existing[0].index + existing[0][0].length, sections[2]?.index ?? text.length).trim()) {
      return text;
    }
    throw new Error(`CHANGELOG.md already contains ${tag} with conflicting release notes`);
  }
  if (!notes) throw new Error("CHANGELOG.md Unreleased section needs release notes");
  return `${text.slice(0, start)}## Unreleased\n\n## ${tag} — ${date}\n\n${notes}\n\n${text.slice(end).trimEnd()}\n`.trimEnd() + "\n";
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  try {
    const [tag, mode = "--preview"] = process.argv.slice(2);
    if (!["--preview", "--write"].includes(mode)) throw new Error("Expected --preview or --write");
    const original = readFileSync("CHANGELOG.md", "utf8");
    const prepared = prepareChangelog(original, tag, new Date().toISOString().slice(0, 10));
    if (mode === "--write") {
      if (prepared !== original) writeFileSync("CHANGELOG.md", prepared);
    } else {
      process.stdout.write(prepared);
    }
  } catch (error) {
    console.error(error.message);
    process.exitCode = 1;
  }
}
