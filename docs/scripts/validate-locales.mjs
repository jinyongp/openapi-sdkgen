import { access, readFile } from "node:fs/promises";
import { resolve } from "node:path";
import { topics, publishedTargets } from "../.vitepress/docs-catalog.mjs";
import { legacyRoutes } from "../.vitepress/legacy-routes.mjs";
import { validateMaterializedDocs } from "./validate-doc-context.mjs";
const defaultDocsRoot = resolve(import.meta.dirname, "..");
export async function validateLocaleStructure({ docsRoot = defaultDocsRoot } = {}) {
  const root = resolve(docsRoot);
  const result = await validateMaterializedDocs(root, publishedTargets);
  for (const route of ["/", "/ko/", ...legacyRoutes.map(entry => entry.route)]) {
    const file = resolve(root, `.${route}${route.endsWith("/") ? "index" : ""}.md`);
    await access(file);
    const source = await readFile(file, "utf8");
    const legacy = legacyRoutes.find(entry => entry.route === route);
    if (legacy && (!/^search: false$/m.test(source) || !source.includes(legacy.destination))) throw new Error(`Invalid historical documentation page: ${route}`);
  }
  return { routes: topics.map(topic => topic.suffix), englishPages: result.pages / 2 + 27, koreanPages: result.pages / 2 + 27 };
}
if (process.argv[1] && resolve(process.argv[1]) === import.meta.filename) {
  const result = await validateLocaleStructure();
  console.log(`ok documentation structure: ${result.englishPages} English, ${result.koreanPages} Korean`);
}
