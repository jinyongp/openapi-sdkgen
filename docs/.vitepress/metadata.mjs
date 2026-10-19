import { topics, sdkTargets, displayLocales, sdkPath, docsBase } from "./docs-catalog.mjs";
import { legacyRoutes } from "./legacy-routes.mjs";
export const siteOrigin = "https://jinyongp.dev";
export const publicURL = route => `${siteOrigin}${docsBase}${route.replace(/^\//, "")}${route.endsWith("/") ? "" : ".html"}`;
export function pageMetadata(relativePath) {
  const route = `/${relativePath.replace(/index\.md$/, "").replace(/\.md$/, "")}`;
  const legacy = legacyRoutes.find(item => item.route === route);
  if (legacy) return [["meta", {name:"robots",content:"noindex,follow"}], ["link",{rel:"canonical",href:publicURL(legacy.destination)}]];
  for (const sdk of sdkTargets) for (const locale of displayLocales) for (const topic of topics) {
    if (sdkPath(sdk,locale,topic.id) !== route) continue;
    return [["link",{rel:"canonical",href:publicURL(route)}], ...displayLocales.map(language=>["link",{rel:"alternate",hreflang:language,href:publicURL(sdkPath(sdk,language,topic.id))}])];
  }
  if (route === "/" || route === "/ko/") return [["link",{rel:"canonical",href:publicURL(route)}], ...["/","/ko/"].map((path,index)=>["link",{rel:"alternate",hreflang:index ? "ko":"en",href:publicURL(path)}])];
  return [];
}
