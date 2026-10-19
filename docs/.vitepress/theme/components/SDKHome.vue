<script setup lang="ts">
import { computed, type ComputedRef } from "vue";
import { useData } from "vitepress";
import { contextDestination, withDocsBase, type SDKTarget } from "../../doc-context";
const { lang } = useData();
const korean: ComputedRef<boolean> = computed((): boolean => lang.value === "ko-KR");
const targets: readonly SDKTarget[] = ["typescript", "golang"];
function destination(sdk: SDKTarget, topic: "getting-started" | "playground" | "reference" | "examples"): string {
  return withDocsBase(contextDestination({ sdk: null, locale: korean.value ? "ko" : "en", topic }, sdk, korean.value ? "ko" : "en").path);
}
</script>

<template>
  <div class="product-home">
    <header class="introduction">
      <h1>openapi-sdkgen</h1>
      <p class="description">{{ korean ? 'OpenAPI 명세에서 SDK 소스를 생성합니다.' : 'Generate SDK source from OpenAPI.' }}</p>
      <p class="summary">{{ korean ? 'OpenAPI 3.0·3.1·3.2 명세를 TypeScript 또는 Go 코드로 변환합니다. 생성된 클라이언트와 모델을 애플리케이션에 넣고 함께 빌드하세요.' : 'Turn OpenAPI 3.0, 3.1 and 3.2 definitions into TypeScript or Go code. Add the generated clients and models to your application and build them together.' }}</p>
      <a class="start-link" href="#start">{{ korean ? '시작하기' : 'Get started' }} <span aria-hidden="true">→</span></a>
    </header>

    <section class="overview" aria-labelledby="overview-title">
      <h2 id="overview-title">{{ korean ? '생성되는 코드' : 'What it generates' }}</h2>
      <ul>
        <li>{{ korean ? 'API 호출에 사용할 클라이언트, 요청·응답 모델과 스키마 검증 코드' : 'API clients, request and response models, and schema validation' }}</li>
        <li>{{ korean ? 'JSON·XML·파일 전송과 SSE·NDJSON 스트림 처리 코드' : 'JSON, XML, file transfer, SSE and NDJSON stream handling' }}</li>
        <li>{{ korean ? '웹훅과 콜백을 받는 처리기, Link를 따라 후속 API를 호출하는 코드' : 'Webhook and callback handlers, and follow-up API calls through Links' }}</li>
      </ul>
      <p>{{ korean ? '필요한 API를 선택해 생성할 수 있습니다. 명세가 바뀌면 SDK를 다시 생성하고, CI에서 생성 결과의 변경 여부를 확인할 수 있습니다.' : 'Select which APIs to generate. When the definition changes, regenerate the SDK and check for changes in CI.' }}</p>
    </section>

    <section class="installation" aria-labelledby="install-title">
      <h2 id="install-title">{{ korean ? 'CLI 설치' : 'Install the CLI' }}</h2>
      <p>macOS / Linux</p>
      <pre><code>curl -fsSL https://jinyongp.dev/openapi-sdkgen/install.sh | sh</code></pre>
      <p>Windows PowerShell</p>
      <pre><code>irm https://jinyongp.dev/openapi-sdkgen/install.ps1 | iex</code></pre>
      <p>{{ korean ? '안내된 설치 경로를 필요에 따라' : 'Add the printed installation directory to' }} <code>PATH</code>{{ korean ? '에 추가한 뒤' : ' if needed, then run' }} <code>openapi-sdkgen --version</code>{{ korean ? '으로 확인하세요.' : '.' }}</p>
      <details>
        <summary>{{ korean ? '다른 설치 방법' : 'Other installation methods' }}</summary>
        <p><a href="https://github.com/jinyongp/openapi-sdkgen/releases">{{ korean ? '실행 파일 직접 다운로드' : 'Download an executable' }}</a></p>
        <p>npm (Node.js 22+): <code>npm install -g openapi-sdkgen</code></p>
        <p>Homebrew: <code>brew install jinyongp/tap/openapi-sdkgen</code></p>
      </details>
    </section>

    <section id="start" class="start" aria-labelledby="start-title">
      <h2 id="start-title">{{ korean ? '문서와 플레이그라운드' : 'Documentation and Playground' }}</h2>
      <p>{{ korean ? '가이드에서 생성 방법과 호출 코드를 확인하세요. 전체 실행 코드는 예제에서 내려받을 수 있습니다.' : 'Read the guides for generation and API calls, or download a complete runnable example.' }}</p>
      <div class="language-links">
        <div v-for="sdk in targets" :key="sdk" class="language-row">
          <h3>{{ sdk === 'golang' ? 'Go' : 'TypeScript' }}</h3>
          <div class="links">
            <a :href="destination(sdk, 'getting-started')" :aria-label="`${sdk === 'golang' ? 'Go' : 'TypeScript'} ${korean ? '시작 가이드' : 'getting started'}`">{{ korean ? '시작 가이드' : 'Get started' }}</a>
            <a :href="destination(sdk, 'examples')" :aria-label="`${sdk === 'golang' ? 'Go' : 'TypeScript'} ${korean ? '예제' : 'examples'}`">{{ korean ? '예제' : 'Examples' }}</a>
            <a :href="destination(sdk, 'playground')" :aria-label="`${sdk === 'golang' ? 'Go' : 'TypeScript'} Playground`">{{ korean ? '플레이그라운드' : 'Playground' }}</a>
            <a :href="destination(sdk, 'reference')" :aria-label="`${sdk === 'golang' ? 'Go' : 'TypeScript'} ${korean ? '레퍼런스' : 'reference'}`">{{ korean ? '레퍼런스' : 'Reference' }}</a>
          </div>
        </div>
      </div>
    </section>
  </div>
</template>

<style scoped>
.product-home { max-width:960px; padding:64px 32px 80px; margin:auto; }
.introduction { padding-bottom:36px; }
h1 { font-size:clamp(32px,5vw,48px); line-height:1.2; font-weight:700; letter-spacing:-.03em; }
.description { margin-top:16px; font-size:24px; line-height:1.5; font-weight:500; }
.summary { max-width:680px; margin-top:16px; }
p, li { font-size:16px; line-height:1.8; color:var(--vp-c-text-2); }
a { color:var(--vp-c-brand-1); text-decoration:underline; text-underline-offset:4px; }
a:hover { color:var(--vp-c-brand-2); }
a:focus-visible { outline:2px solid var(--vp-c-brand-1); outline-offset:5px; }
.start-link { display:inline-block; margin-top:20px; font-weight:600; }
section { padding:28px 0; border-top:1px solid var(--vp-c-divider); }
h2 { margin-bottom:16px; font-size:22px; font-weight:600; line-height:1.4; }
ul { padding-left:22px; list-style:disc; margin-bottom:16px; }
li + li { margin-top:6px; }
pre { padding:16px 20px; border-radius:6px; background:var(--vp-c-bg-soft); font-size:14px; line-height:1.7; overflow-x:auto; }
code { font-family:var(--vp-font-family-mono); }
.installation p { margin:12px 0; font-size:14px; }
.installation summary { cursor:pointer; color:var(--vp-c-text-2); font-size:14px; }
.start { scroll-margin-top:88px; padding-bottom:0; }
.language-links { margin-top:20px; }
.language-row { display:flex; align-items:baseline; gap:32px; padding:16px 0; }
.language-row + .language-row { border-top:1px solid var(--vp-c-divider); }
h3 { min-width:110px; font-size:17px; font-weight:600; }
.links { display:flex; flex-wrap:wrap; gap:12px 28px; font-size:14px; }
:lang(ko) :is(h1,h2,p,li) { word-break:keep-all; overflow-wrap:break-word; }
@media (max-width:639px) {
  .product-home { padding:36px 24px 48px; }
  .description { font-size:21px; }
  .language-row { flex-direction:column; gap:10px; }
  .links { gap:12px 24px; }
}
</style>
