<script setup lang="ts">
import { computed, onMounted, ref, type ComputedRef, type Ref } from "vue";
import { useData } from "vitepress";
import { contextDestination, readSDKPreference, withDocsBase, type SDKTarget } from "../../doc-context";
const { lang } = useData();
const korean: ComputedRef<boolean> = computed((): boolean => lang.value === "ko-KR");
const preference: Ref<SDKTarget | null> = ref(null);
onMounted((): void => { try { preference.value = readSDKPreference(window.localStorage); } catch { /* Optional preference. */ } });
function destination(sdk: SDKTarget): string { return withDocsBase(contextDestination({ sdk: null, locale: korean.value ? "ko" : "en", topic: "getting-started" }, sdk, korean.value ? "ko" : "en").path); }
</script>
<template>
  <section class="sdk-home">
    <h1>{{ korean ? 'OpenAPI로 애플리케이션 SDK 만들기' : 'Application SDKs from OpenAPI' }}</h1>
    <p>{{ korean ? '사용할 SDK 언어를 선택하세요. 문서·예제·검색·플레이그라운드가 선택한 언어에 맞춰 표시됩니다.' : 'Choose your SDK language. Guides, examples, search and the Playground follow your selection.' }}</p>
    <div class="sdk-cards">
      <a :href="destination('typescript')"><strong>TypeScript</strong><span>{{ korean ? '타입이 있는 Fetch 호출·스트림·수신 처리기' : 'Typed Fetch calls, streams and inbound handlers' }}</span></a>
      <a :href="destination('golang')"><strong>Go</strong><span>{{ korean ? 'context 기반 호출·타입이 있는 모델·net/http 처리기' : 'Context-aware calls, typed models and net/http handlers' }}</span></a>
    </div>
    <p v-if="preference"><a :href="destination(preference)">{{ korean ? '이어서 보기' : 'Continue with' }} · {{ preference === 'golang' ? 'Go' : 'TypeScript' }}</a></p>
  </section>
</template>
<style scoped>
.sdk-home { max-width:960px; padding:64px 24px; margin:auto; }
h1 { font-size:clamp(26px,4vw,46px); line-height:1.2; font-weight:700; margin-bottom:24px; }
p { margin:20px 0; color:var(--vp-c-text-2); }
.sdk-cards { display:grid; grid-template-columns:repeat(auto-fit,minmax(min(100%,260px),1fr)); gap:20px; }
a { color:var(--vp-c-brand-1); }
.sdk-cards a { display:flex; flex-direction:column; align-items:flex-start; gap:12px; padding:24px; background:var(--vp-c-bg-soft); border:1px solid var(--vp-c-divider); border-radius:12px; }
strong { font-size:24px; line-height:32px; } span { line-height:1.6; color:var(--vp-c-text-2); }
a:focus-visible { outline:2px solid var(--vp-c-brand-1); outline-offset:4px; }
</style>
