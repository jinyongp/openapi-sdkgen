import { topics, sdkPath } from "./docs-catalog.mjs";

export function buildSDKNavigation(locale, sdk) {
  const labels = locale === "ko" ? {guide:"사용 가이드",examples:"예제",reference:"참조"} : {guide:"Guides",examples:"Examples",reference:"Reference"};
  return {
    nav: ["getting-started", "reference", "playground"].map(id => ({text:topics.find(t=>t.id===id).labels[locale],link:sdkPath(sdk,locale,id)})),
    sidebar: { [sdkPath(sdk,locale)]: Object.entries(labels).map(([group,text])=>({text,items:topics.filter(t=>t.group===group).map(t=>({text:t.labels[locale],link:sdkPath(sdk,locale,t.id)}))})) },
  };
}
