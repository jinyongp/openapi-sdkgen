import { readdir, readFile } from "node:fs/promises";
import { dirname, relative, resolve, sep } from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";

import {
  docsNavigationLinks,
  localeParityExceptions,
  publicDocRoutes,
} from "../.vitepress/navigation.mjs";

const scriptDirectory = dirname(fileURLToPath(import.meta.url));
const defaultDocsRoot = resolve(scriptDirectory, "..");
const publicDirectories = ["guide", "examples", "reference"];
const publicRootFiles = ["index.md", "playground.md"];

export async function validateLocaleStructure({ docsRoot = defaultDocsRoot } = {}) {
  const root = resolve(docsRoot);
  const english = await collectLocalePages(root, "en");
  const korean = await collectLocalePages(root, "ko");
  const exceptions = new Set(localeParityExceptions);
  const expected = new Set(publicDocRoutes.filter((route) => !exceptions.has(route)));
  const errors = [];

  compareRouteSet(errors, "English pages", expected, routeSet(english, exceptions));
  compareRouteSet(errors, "Korean pages", expected, routeSet(korean, exceptions));
  compareRouteSet(errors, "English/Korean page parity", routeSet(english, exceptions), routeSet(korean, exceptions));

  validateNavigation(errors, "en", expected);
  validateNavigation(errors, "ko", expected);

  const publicFiles = new Map([...english.files, ...korean.files]);
  for (const page of [...english.pages, ...korean.pages]) {
    await validateMarkdownLinks(errors, root, page, publicFiles);
  }

  if (errors.length !== 0) {
    throw new Error(["documentation locale validation failed:", ...errors.sort().map((value) => `- ${value}`)].join("\n"));
  }
  return {
    routes: [...expected].sort(),
    englishPages: english.pages.length,
    koreanPages: korean.pages.length,
  };
}

async function collectLocalePages(docsRoot, locale) {
  const localeRoot = locale === "ko" ? resolve(docsRoot, "ko") : docsRoot;
  const pages = [];
  for (const name of publicRootFiles) {
    pages.push(pageRecord(docsRoot, localeRoot, locale, resolve(localeRoot, name)));
  }
  for (const directory of publicDirectories) {
    const files = await markdownFiles(resolve(localeRoot, directory));
    for (const file of files) {
      pages.push(pageRecord(docsRoot, localeRoot, locale, file));
    }
  }
  const routes = new Map();
  const files = new Map();
  for (const page of pages) {
    if (routes.has(page.route)) {
      throw new Error(`duplicate ${locale} documentation route ${page.route}`);
    }
    routes.set(page.route, page);
    files.set(page.file, page);
  }
  return { pages, routes, files };
}

async function markdownFiles(directory) {
  const entries = await readdir(directory, { withFileTypes: true });
  const files = [];
  for (const entry of entries.sort((left, right) => left.name.localeCompare(right.name))) {
    const path = resolve(directory, entry.name);
    if (entry.isDirectory()) files.push(...(await markdownFiles(path)));
    else if (entry.isFile() && entry.name.endsWith(".md")) files.push(path);
  }
  return files;
}

function pageRecord(docsRoot, localeRoot, locale, file) {
  const relativePath = toPosix(relative(localeRoot, file));
  return {
    locale,
    file,
    display: toPosix(relative(docsRoot, file)),
    route: markdownPathToRoute(relativePath),
  };
}

function markdownPathToRoute(path) {
  if (path === "index.md") return "/";
  if (path.endsWith("/index.md")) return `/${path.slice(0, -"index.md".length)}`;
  return `/${path.slice(0, -".md".length)}`;
}

function routeSet(locale, exceptions) {
  return new Set([...locale.routes.keys()].filter((route) => !exceptions.has(route)));
}

function compareRouteSet(errors, label, expected, actual) {
  for (const route of [...expected].sort()) {
    if (!actual.has(route)) errors.push(`${label} is missing ${route}`);
  }
  for (const route of [...actual].sort()) {
    if (!expected.has(route)) errors.push(`${label} has undeclared route ${route}`);
  }
}

function validateNavigation(errors, locale, expected) {
  for (const link of docsNavigationLinks(locale)) {
    const route = normalizeSiteRoute(link);
    if (!expected.has(route)) errors.push(`${locale} navigation points to undeclared page ${link}`);
  }
}

async function validateMarkdownLinks(errors, docsRoot, page, publicFiles) {
  const source = await readFile(page.file, "utf8");
  const pattern = /\[([^\]]*)\]\(([^)]+)\)/g;
  for (const match of source.matchAll(pattern)) {
    if (match.index !== undefined && source[match.index - 1] === "!") continue;
    const destination = linkDestination(match[2]);
    if (destination === "" || destination.startsWith("#") || isExternalLink(destination)) continue;
    const withoutFragment = destination.split(/[?#]/, 1)[0];
    if (withoutFragment === "") continue;

    if (withoutFragment.startsWith("/")) {
      const route = normalizeSiteRoute(withoutFragment);
      if (!publicDocRoutes.includes(route)) {
        errors.push(`${page.display} links to missing documentation route ${destination}`);
      }
      continue;
    }

    if (!withoutFragment.toLowerCase().endsWith(".md")) continue;
    const target = resolve(dirname(page.file), withoutFragment);
    if (!isWithin(docsRoot, target) || !publicFiles.has(target)) {
      errors.push(`${page.display} links to missing public documentation page ${destination}`);
    }
  }
}

function linkDestination(value) {
  const trimmed = value.trim();
  if (trimmed.startsWith("<")) {
    const end = trimmed.indexOf(">");
    return end < 0 ? trimmed : trimmed.slice(1, end);
  }
  const title = trimmed.search(/\s+["']/);
  return title < 0 ? trimmed : trimmed.slice(0, title);
}

function isExternalLink(value) {
  return value.startsWith("//") || /^[a-z][a-z0-9+.-]*:/i.test(value);
}

function normalizeSiteRoute(value) {
  let route = value.split(/[?#]/, 1)[0];
  if (route.startsWith("/openapi-sdkgen/")) route = route.slice("/openapi-sdkgen".length);
  if (route === "/ko" || route === "/ko/") return "/";
  if (route.startsWith("/ko/")) route = route.slice(3);
  if (route.endsWith(".md")) route = markdownPathToRoute(route.slice(1));
  return route;
}

function isWithin(root, path) {
  const value = relative(resolve(root), resolve(path));
  return value === "" || (!value.startsWith(`..${sep}`) && value !== "..");
}

function toPosix(value) {
  return value.split(sep).join("/");
}

if (process.argv[1] !== undefined && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) {
  try {
    const result = await validateLocaleStructure();
    console.log(
      `ok documentation locale structure: ${result.routes.length} routes, ${result.englishPages} English, ${result.koreanPages} Korean`,
    );
  } catch (error) {
    console.error(error instanceof Error ? error.message : String(error));
    process.exitCode = 1;
  }
}
