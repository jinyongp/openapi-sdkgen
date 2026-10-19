import { legacyRoutes } from "./legacy-routes.mjs";
interface LegacyEntry { readonly route: string; readonly destination: string; readonly anchors: readonly string[]; readonly fragments: Readonly<Record<string, string>>; }
const entries: readonly LegacyEntry[] = legacyRoutes;
export function legacyDestination(route: string, href: string): string {
  const entry: LegacyEntry | undefined = entries.find((candidate: LegacyEntry): boolean => candidate.route === route);
  if (!entry) throw new Error(`Unknown historical route: ${route}`);
  const url: URL = new URL(href, "https://docs.invalid");
  let decoded: string = "";
  try { decoded = decodeURIComponent(url.hash.slice(1)).normalize("NFD"); } catch { /* Drop malformed fragments. */ }
  const fragment: string | undefined = entry.anchors.find((anchor: string): boolean => anchor.normalize("NFD") === decoded);
  if (route === "/playground" || route === "/ko/playground") {
    const query: URLSearchParams = new URLSearchParams();
    for (const key of ["example", "with", "go-import-path", "go-package"]) if (url.searchParams.has(key)) query.set(key, url.searchParams.get(key)!);
    const sdk: string = url.searchParams.get("target") === "golang" ? "golang" : "typescript";
    return `${route.startsWith("/ko/") ? "/ko" : ""}/${sdk}/playground${query.size ? `?${query}` : ""}`;
  }
  return fragment ? entry.fragments[fragment] ?? `${entry.destination}#${fragment}` : entry.destination;
}
