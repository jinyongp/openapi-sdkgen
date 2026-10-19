import { access, readdir, readFile, stat } from "node:fs/promises";
import { resolve, dirname } from "node:path";
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
    for (const path of expected) {
      await access(resolve(docsRoot, path));
      const source = await readFile(resolve(docsRoot, path), "utf8");
      for (const match of source.matchAll(/\[[^\]]*\]\(([^)\s]+)\)/g)) {
        const link = match[1].split(/[?#]/)[0];
        if (!link || /^[a-z][a-z0-9+.-]*:|^\/\//i.test(link)) continue;
        let file;
        if (link.startsWith("/")) {
          const route = link.replace(/^\/openapi-sdkgen/, "");
          const sdkRoute = route.replace(/^\//, "");
          file = /^(ko\/)?(typescript|golang)(\/|$)/.test(sdkRoute)
            ? resolve(docsRoot, `${sdkRoute.endsWith("/") ? sdkRoute + "index" : sdkRoute}.md`)
            : resolve(docsRoot, "public", `.${route}`);
        } else if (link.endsWith(".md")) file = resolve(docsRoot, dirname(path), link);
        if (file && !(await stat(file).catch(() => null))?.isFile()) throw new Error(`${path}: missing link ${link}`);
      }
      count++;
    }
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
  const result = await validateMaterializedDocs(resolve(import.meta.dirname, ".."));
  console.log(`ok documentation context: ${result.pages} SDK pages`);
}
