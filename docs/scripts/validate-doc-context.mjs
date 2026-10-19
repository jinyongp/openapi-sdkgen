import { access, readdir } from "node:fs/promises";
import { resolve } from "node:path";
import { pathToFileURL } from "node:url";
import { topics, sdkTargets, displayLocales, sdkPath, publishedTargets } from "../.vitepress/docs-catalog.mjs";
import { legacyRoutes } from "../.vitepress/legacy-routes.mjs";

export function validateCatalog(entries = topics, legacy = legacyRoutes) {
  const ids = new Set();
  const paths = new Set();
  for (const topic of entries) {
    if (ids.has(topic.id)) throw new Error(`Duplicate documentation topic ${topic.id}`);
    ids.add(topic.id);
    for (const locale of displayLocales) {
      if (!topic.labels[locale]) throw new Error(`Missing ${locale} label: ${topic.id}`);
      for (const sdk of topic.targets) {
        if (!sdkTargets.includes(sdk)) throw new Error(`Unknown SDK: ${sdk}`);
        const path = `${locale}/${sdk}${topic.suffix}`;
        if (paths.has(path)) throw new Error(`Duplicate documentation path ${path}`);
        paths.add(path);
      }
    }
  }
  const historical = new Set();
  for (const entry of legacy) {
    if (historical.has(entry.route)) throw new Error(`Duplicate legacy route ${entry.route}`);
    historical.add(entry.route);
    if (sdkPath(entry.sdk, entry.locale, entry.topic) !== entry.destination) throw new Error(`Invalid legacy destination: ${entry.route}`);
  }
}

export async function validateMaterializedDocs(docsRoot, targets = publishedTargets) {
  validateCatalog();
  let count = 0;
  for (const locale of displayLocales) for (const sdk of targets) {
    const prefix = locale === "ko" ? `ko/${sdk}` : sdk;
    const expected = new Set(topics.filter(topic => topic.targets.includes(sdk)).map(topic => `${prefix}${topic.suffix === "/" || topic.suffix.endsWith("/") ? topic.suffix + "index" : topic.suffix}.md`));
    const actual = await markdownFiles(docsRoot, prefix);
    for (const path of expected) { await access(resolve(docsRoot, path)); count++; }
    for (const path of actual) if (!expected.has(path)) throw new Error(`Undeclared SDK documentation page: ${path}`);
  }
  return { pages: count, targets };
}
async function markdownFiles(root, directory) {
  const files = [];
  for (const entry of await readdir(resolve(root, directory), { withFileTypes: true })) {
    const path = `${directory}/${entry.name}`;
    if (entry.isDirectory()) files.push(...await markdownFiles(root, path));
    else if (entry.name.endsWith(".md")) files.push(path);
  }
  return files;
}
if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) {
  validateCatalog();
  console.log("ok documentation context catalog");
}
