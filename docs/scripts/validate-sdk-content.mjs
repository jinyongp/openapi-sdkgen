import { readFile } from "node:fs/promises";
import { resolve } from "node:path";
import { topics, sdkPath, publishedTargets } from "../.vitepress/docs-catalog.mjs";
import { validateMaterializedDocs } from "./validate-doc-context.mjs";
const root = resolve(import.meta.dirname, "..");
export async function validateSDKContent(targets = publishedTargets) {
  const result = await validateMaterializedDocs(root, targets);
  for (const sdk of targets) for (const topic of topics) {
    const contents = [];
    for (const locale of ["en", "ko"]) {
      const route = sdkPath(sdk, locale, topic.id);
      const path = `${route.endsWith("/") ? route + "index" : route}.md`;
      const source = await readFile(resolve(root, `.${path}`), "utf8");
      contents.push([...source.matchAll(/```([^\n]*)\n([\s\S]*?)```/g)].map(m => `${m[1]}\n${m[2]}`));
      if (/next release|source preview|미출시|다음 릴리스|소스 미리보기/i.test(source)) throw new Error(`Unreleased product copy: ${path}`);
      if (sdk === "typescript" && /--go-import-path|--go-package|\[golang\]/.test(source)) throw new Error(`Go options in TypeScript page: ${path}`);
      if (sdk === "golang" && /```(?:ts|typescript|js|javascript)\b|\$operations|\$routes|createClient\(/.test(source)) throw new Error(`TypeScript consumer API in Go page: ${path}`);
    }
    if (JSON.stringify(contents[0]) !== JSON.stringify(contents[1])) throw new Error(`Code examples differ by locale: ${sdk}/${topic.id}`);
  }
  return result;
}
if (process.argv[1] && resolve(process.argv[1]) === import.meta.filename) {
  const result = await validateSDKContent(process.argv.includes("--all") ? ["typescript", "golang"] : publishedTargets);
  console.log(`ok SDK content: ${result.pages} pages, paired examples and release copy`);
}
