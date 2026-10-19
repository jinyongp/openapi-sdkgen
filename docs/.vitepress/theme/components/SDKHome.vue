<script setup lang="ts">
import { computed, type ComputedRef } from "vue";
import { useData, useRoute } from "vitepress";
import { contextDestination, parseDocContext, withDocsBase, type SDKTarget } from "../../doc-context";
const { lang } = useData();
const route: ReturnType<typeof useRoute> = useRoute();
const korean: ComputedRef<boolean> = computed((): boolean => lang.value === "ko-KR");
const selected: ComputedRef<SDKTarget | null> = computed((): SDKTarget | null => parseDocContext(route.path)?.sdk ?? null);
const targets: ComputedRef<readonly SDKTarget[]> = computed((): readonly SDKTarget[] => selected.value ? [selected.value] : ["typescript", "golang"]);
const heading: ComputedRef<string> = computed((): string => selected.value ? `${selected.value === "golang" ? "Go" : "TypeScript"} SDK` : "openapi-sdkgen");
const description: ComputedRef<string> = computed((): string => {
  const target: string = selected.value ? `${selected.value === "golang" ? "Go" : "TypeScript"} ` : "TypeScript·Go ";
  return korean.value
    ? `OpenAPI 3.0·3.1·3.2 명세에서 ${target}SDK 소스를 생성합니다.`
    : `Generate ${selected.value ? target : "TypeScript and Go "}SDK source from OpenAPI 3.0, 3.1 and 3.2.`;
});
function destination(sdk: SDKTarget, topic: "getting-started" | "playground" | "reference" | "examples"): string {
  return withDocsBase(contextDestination({ sdk: null, locale: korean.value ? "ko" : "en", topic }, sdk, korean.value ? "ko" : "en").path);
}
</script>

<template>
  <div class="product-home">
    <header class="introduction">
      <h1>{{ heading }}</h1>
      <p class="description">{{ description }}</p>
    </header>

    <nav id="start" :aria-label="korean ? '문서와 플레이그라운드' : 'Documentation and Playground'">
      <div class="language-links">
        <div v-for="sdk in targets" :key="sdk" class="language-row">
          <h2 v-if="!selected">{{ sdk === 'golang' ? 'Go' : 'TypeScript' }}</h2>
          <div class="links">
            <a :href="destination(sdk, 'getting-started')" :aria-label="`${sdk === 'golang' ? 'Go' : 'TypeScript'} ${korean ? '시작 가이드' : 'getting started'}`">{{ korean ? '시작 가이드' : 'Get started' }}</a>
            <a :href="destination(sdk, 'examples')" :aria-label="`${sdk === 'golang' ? 'Go' : 'TypeScript'} ${korean ? '예제' : 'examples'}`">{{ korean ? '예제' : 'Examples' }}</a>
            <a :href="destination(sdk, 'playground')" :aria-label="`${sdk === 'golang' ? 'Go' : 'TypeScript'} Playground`">{{ korean ? '플레이그라운드' : 'Playground' }}</a>
            <a :href="destination(sdk, 'reference')" :aria-label="`${sdk === 'golang' ? 'Go' : 'TypeScript'} ${korean ? '레퍼런스' : 'reference'}`">{{ korean ? '레퍼런스' : 'Reference' }}</a>
          </div>
        </div>
      </div>
    </nav>
  </div>
</template>

<style scoped>
.product-home { max-width:960px; padding:64px 32px 80px; margin:auto; }
h1 { font-size:32px; line-height:1.3; font-weight:600; }
.description { max-width:680px; margin-top:12px; font-size:16px; line-height:1.8; color:var(--vp-c-text-2); }
a { color:var(--vp-c-brand-1); text-decoration:underline; text-underline-offset:4px; }
a:hover { color:var(--vp-c-brand-2); }
a:focus-visible { outline:2px solid var(--vp-c-brand-1); outline-offset:5px; }
nav { margin-top:28px; border-top:1px solid var(--vp-c-divider); scroll-margin-top:88px; }
.language-row { display:flex; align-items:baseline; gap:32px; padding:20px 0; }
.language-row + .language-row { border-top:1px solid var(--vp-c-divider); }
h2 { min-width:110px; font-size:17px; font-weight:600; }
.links { display:flex; flex-wrap:wrap; gap:12px 28px; font-size:14px; }
:lang(ko) :is(h1,h2,p) { word-break:keep-all; overflow-wrap:break-word; }
@media (max-width:639px) {
  .product-home { padding:36px 24px 48px; }
  .language-row { flex-direction:column; gap:10px; }
  .links { gap:12px 24px; }
}
</style>
