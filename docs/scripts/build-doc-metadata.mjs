import { writeFile } from "node:fs/promises";
import { resolve } from "node:path";
import { topics, sdkTargets, displayLocales, sdkPath } from "../.vitepress/docs-catalog.mjs";
import { publicURL } from "../.vitepress/metadata.mjs";
const entries = [
  ...["/", "/ko/"].map(route=>({route,alternates:["/","/ko/"]})),
  ...sdkTargets.flatMap(sdk=>displayLocales.flatMap(locale=>topics.map(topic=>({route:sdkPath(sdk,locale,topic.id),alternates:displayLocales.map(language=>sdkPath(sdk,language,topic.id))})))),
];
const xml = '<?xml version="1.0" encoding="UTF-8"?>\n<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9" xmlns:xhtml="http://www.w3.org/1999/xhtml">\n'+entries.map(entry=>`<url><loc>${publicURL(entry.route)}</loc>${entry.alternates.map((route,index)=>`<xhtml:link rel="alternate" hreflang="${index ? "ko":"en"}" href="${publicURL(route)}"/>`).join("")}</url>`).join("\n")+'\n</urlset>\n';
await writeFile(resolve(import.meta.dirname,"../public/sitemap.xml"),xml);
console.log(`ok documentation sitemap: ${entries.length} canonical pages`);
