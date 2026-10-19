import { docsBase, documentationScopes, sdkPath, topics as catalogTopics } from "./docs-catalog.mjs";

export type SDKTarget = "typescript" | "golang";
export type DisplayLocale = "en" | "ko";
export interface DocContext { readonly sdk: SDKTarget | null; readonly locale: DisplayLocale; readonly topic: string; }
export interface DocDestination { readonly path: string; readonly fallback: boolean; }
import type { PreferenceStorage } from "./theme/playground/preferences.ts";
export type { PreferenceStorage } from "./theme/playground/preferences.ts";
interface DocTopic { readonly id: string; readonly suffix: string; readonly targets: readonly SDKTarget[]; }
interface DocScope { readonly key: string; readonly sdk: SDKTarget | null; readonly locale: DisplayLocale; }
const topics: readonly DocTopic[] = catalogTopics;
const scopes: readonly DocScope[] = documentationScopes;
const sdkPreferenceKey: string = "openapi-sdkgen.docs.sdk.v1";

export function isSDKTarget(value: unknown): value is SDKTarget { return value === "typescript" || value === "golang"; }
export function parseDocContext(href: string): DocContext | null {
  let pathname: string;
  try { pathname = new URL(href, "https://docs.invalid").pathname; } catch { return null; }
  if (pathname === docsBase.slice(0, -1)) pathname = "/";
  else if (pathname.startsWith(docsBase)) pathname = `/${pathname.slice(docsBase.length)}`;
  pathname = pathname.replace(/\/index\.html$/, "/").replace(/\.html$/, "");
  if (pathname === "/" || pathname === "/ko" || pathname === "/ko/") return { sdk: null, locale: pathname.startsWith("/ko") ? "ko" : "en", topic: "home" };
  for (const scope of scopes) {
    if (!scope.sdk) continue;
    const prefix: string = `/${scope.key}`;
    if (pathname !== prefix && !pathname.startsWith(`${prefix}/`)) continue;
    const suffix: string = pathname.slice(prefix.length) || "/";
    const topic: DocTopic | undefined = topics.find((item: DocTopic): boolean => item.suffix.replace(/\/$/, "") === suffix.replace(/\/$/, ""));
    if (!topic) return null;
    return { sdk: scope.sdk, locale: scope.locale, topic: topic.id };
  }
  return null;
}

export function contextDestination(current: DocContext, sdk: SDKTarget, locale: DisplayLocale): DocDestination {
  const topic: DocTopic | undefined = topics.find((item: DocTopic): boolean => item.id === current.topic && item.targets.includes(sdk));
  return { path: sdkPath(sdk, locale, topic?.id ?? "getting-started"), fallback: !topic };
}
export function withDocsBase(path: string): string { return path.startsWith(docsBase) ? path : `${docsBase}${path.replace(/^\//, "")}`; }
export function readSDKPreference(storage: PreferenceStorage | undefined): SDKTarget | null {
  try { const value: string | null | undefined = storage?.getItem(sdkPreferenceKey); return isSDKTarget(value) ? value : null; } catch { return null; }
}
export function writeSDKPreference(storage: PreferenceStorage | undefined, sdk: SDKTarget): void {
  try { storage?.setItem(sdkPreferenceKey, sdk); } catch { /* URL navigation remains available when storage is blocked. */ }
}
