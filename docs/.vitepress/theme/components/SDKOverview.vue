<script setup lang="ts">
import { computed, type ComputedRef } from "vue";
import { useData } from "vitepress";
import { contextDestination, withDocsBase, type SDKTarget } from "../../doc-context";

interface SDKOverviewProps { readonly sdk: SDKTarget; }
const props: SDKOverviewProps = defineProps<SDKOverviewProps>();
const { lang } = useData();
const korean: ComputedRef<boolean> = computed((): boolean => lang.value === "ko-KR");
const name: ComputedRef<string> = computed((): string => props.sdk === "golang" ? "Go" : "TypeScript");
function destination(topic: "getting-started" | "playground" | "reference" | "examples"): string {
  return withDocsBase(contextDestination({ sdk: props.sdk, locale: korean.value ? "ko" : "en", topic }, props.sdk, korean.value ? "ko" : "en").path);
}
</script>

<template>
  <div class="sdk-home">
    <header>
      <h1>{{ name }} SDK</h1>
      <p>{{ korean ? `OpenAPI 3.0·3.1·3.2 명세에서 ${name} SDK 소스를 생성합니다.` : `Generate ${name} SDK source from OpenAPI 3.0, 3.1 and 3.2.` }}</p>
    </header>
    <nav id="start" :aria-label="korean ? '문서와 플레이그라운드' : 'Documentation and Playground'">
      <a :href="destination('getting-started')">{{ korean ? '시작 가이드' : 'Get started' }}</a>
      <a :href="destination('examples')">{{ korean ? '예제' : 'Examples' }}</a>
      <a :href="destination('playground')">{{ korean ? '플레이그라운드' : 'Playground' }}</a>
      <a :href="destination('reference')">{{ korean ? '레퍼런스' : 'Reference' }}</a>
    </nav>
  </div>
</template>

<style scoped>
.sdk-home { max-width:960px; padding:64px 32px 80px; margin:auto; }
h1 { font-size:32px; line-height:1.3; font-weight:600; }
p { max-width:680px; margin-top:12px; font-size:16px; line-height:1.8; color:var(--vp-c-text-2); }
nav { display:flex; flex-wrap:wrap; gap:12px 28px; margin-top:28px; padding:20px 0; border-top:1px solid var(--vp-c-divider); font-size:14px; scroll-margin-top:88px; }
a { color:var(--vp-c-brand-1); text-decoration:underline; text-underline-offset:4px; }
a:hover { color:var(--vp-c-brand-2); }
a:focus-visible { outline:2px solid var(--vp-c-brand-1); outline-offset:5px; }
:lang(ko) :is(h1,p) { word-break:keep-all; overflow-wrap:break-word; }
@media (max-width:639px) {
  .sdk-home { padding:36px 24px 48px; }
  nav { gap:12px 24px; }
}
</style>
