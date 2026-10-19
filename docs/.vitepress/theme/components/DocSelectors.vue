<script setup lang="ts">
import { computed, nextTick, watch, type ComputedRef } from "vue";
import { useRoute, useRouter } from "vitepress";
import { invalidateSession, playgroundSession, preserveContextNavigation, refreshSessionNavigation } from "../playground/session";
import { publishedTargets } from "../../docs-catalog.mjs";
import { contextDestination, isSDKTarget, parseDocContext, withDocsBase, writeSDKPreference, type DocContext, type DisplayLocale, type SDKTarget } from "../../doc-context";

interface SelectorProps { readonly placement: "desktop" | "mobile"; }
const props: SelectorProps = defineProps<SelectorProps>();
const route: ReturnType<typeof useRoute> = useRoute();
const router: ReturnType<typeof useRouter> = useRouter();
const context: ComputedRef<DocContext> = computed((): DocContext => parseDocContext(route.path) ?? { sdk: null, locale: route.path.includes("/ko/") ? "ko" : "en", topic: "home" });
const korean: ComputedRef<boolean> = computed((): boolean => context.value.locale === "ko");
const hintID: string = `docs-selection-hint-${props.placement}`;
const targets: readonly SDKTarget[] = publishedTargets;

function remember(sdk: SDKTarget): void {
  try { writeSDKPreference(window.localStorage, sdk); } catch { /* Storage is optional. */ }
}
async function navigate(sdk: SDKTarget | null, locale: DisplayLocale): Promise<void> {
  const current: DocContext = context.value;
  if (sdk) remember(sdk);
  if (current.topic === "playground" && sdk !== current.sdk) { invalidateSession(); playgroundSession.artifacts.value = []; }
  const destination: string = sdk ? contextDestination(current, sdk, locale).path : locale === "ko" ? "/ko/" : "/";
  const query: URLSearchParams = new URLSearchParams(current.topic === "playground" ? window.location.search : "");
  query.delete("target");
  if (sdk === "typescript") { query.delete("go-import-path"); query.delete("go-package"); }
  const suffix: string = query.size ? `?${query.toString()}` : "";
  if (current.topic === "playground") preserveContextNavigation(withDocsBase(destination));
  await router.go(`${withDocsBase(destination)}${suffix}`);
  await nextTick();
  if (current.topic === "playground") refreshSessionNavigation();
  if (current.topic === "playground" && props.placement === "mobile") {
    const menu: HTMLButtonElement | null = document.querySelector(".VPNavBarHamburger");
    if (menu?.getAttribute("aria-expanded") !== "true") menu?.click();
    await nextTick();
  }
  const focusTarget: HTMLElement | null = document.querySelector(current.topic === "playground" ? `[data-doc-selectors="${props.placement}"] select` : "main h1, .VPHome h1");
  if (focusTarget) { if (focusTarget.tagName !== "SELECT") focusTarget.setAttribute("tabindex", "-1"); focusTarget.focus(); }
}
function selectSDK(event: Event): void {
  const sdk: string = (event.target as HTMLSelectElement).value;
  if (isSDKTarget(sdk)) void navigate(sdk, context.value.locale);
}
function selectLocale(event: Event): void {
  const locale: string = (event.target as HTMLSelectElement).value;
  if (locale === "en" || locale === "ko") void navigate(context.value.sdk, locale);
}
watch((): SDKTarget | null => context.value.sdk, (sdk: SDKTarget | null): void => { if (sdk && typeof window !== "undefined") remember(sdk); }, { immediate: true });
</script>

<template>
  <div class="doc-selectors" :data-doc-selectors="placement">
    <label :for="`sdk-${placement}`">SDK
      <select :id="`sdk-${placement}`" :value="context.sdk ?? ''" :aria-describedby="hintID" @change="selectSDK">
        <option v-if="!context.sdk" value="" disabled>{{ korean ? '선택' : 'Choose' }}</option>
        <option v-for="sdk in targets" :key="sdk" :value="sdk">{{ sdk === 'golang' ? 'Go' : 'TypeScript' }}</option>
      </select>
    </label>
    <label :for="`locale-${placement}`">{{ korean ? '표시 언어' : 'Language' }}
      <select :id="`locale-${placement}`" :value="context.locale" :aria-describedby="hintID" @change="selectLocale">
        <option value="en">English</option><option value="ko">한국어</option>
      </select>
    </label>
    <span :id="hintID" class="selection-hint">{{ korean ? '선택하면 해당 문서로 이동합니다.' : 'Selecting opens the corresponding documentation.' }}</span>
  </div>
</template>

<style scoped>
.doc-selectors { display:flex; gap:12px; align-items:center; padding:8px 12px; flex-wrap:wrap; }
label { display:flex; align-items:center; gap:6px; font-size:12px; }
select { max-width:100%; background:var(--vp-c-bg-soft); border:1px solid var(--vp-c-divider); border-radius:6px; padding:4px 8px; color:var(--vp-c-text-1); font:inherit; appearance:auto; }
select:focus-visible { outline:2px solid var(--vp-c-brand-1); outline-offset:3px; }
.selection-hint { position:absolute; width:1px; height:1px; overflow:hidden; clip-path:inset(50%); }
[data-doc-selectors="desktop"] { display:none; }
@media (min-width: 960px) { [data-doc-selectors="desktop"] { display:flex; flex:0 0 auto; flex-wrap:nowrap; gap:10px; padding-block:2px; } [data-doc-selectors="desktop"] label { display:grid; gap:0; line-height:1.4; } [data-doc-selectors="mobile"] { display:none; } }
</style>
