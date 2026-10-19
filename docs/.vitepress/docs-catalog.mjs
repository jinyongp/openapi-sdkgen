// Route metadata shared by navigation, context controls and validators.
/** @type {readonly import("./doc-contracts").SDKTarget[]} */
export const sdkTargets = Object.freeze(["typescript", "golang"]);
/** @type {readonly import("./doc-contracts").DisplayLocale[]} */
export const displayLocales = Object.freeze(["en", "ko"]);
export const docsBase = "/openapi-sdkgen/";
// Both SDKs ship together with this documentation.
export const publishedTargets = sdkTargets;
/** @type {readonly import("./doc-contracts").DocTopic[]} */
export const topics = Object.freeze([
  {
    "id": "home",
    "suffix": "/",
    "labels": {
      "en": "Home",
      "ko": "홈"
    },
    "group": "home",
    "targets": [
      "typescript",
      "golang"
    ]
  },
  {
    "id": "getting-started",
    "suffix": "/guide/getting-started",
    "labels": {
      "en": "First SDK",
      "ko": "첫 SDK 만들기"
    },
    "group": "guide",
    "targets": [
      "typescript",
      "golang"
    ]
  },
  {
    "id": "generate",
    "suffix": "/guide/generate",
    "labels": {
      "en": "Generate and verify",
      "ko": "생성과 검증"
    },
    "group": "guide",
    "targets": [
      "typescript",
      "golang"
    ]
  },
  {
    "id": "inspect",
    "suffix": "/guide/inspect",
    "labels": {
      "en": "Find APIs",
      "ko": "API 찾기"
    },
    "group": "guide",
    "targets": [
      "typescript",
      "golang"
    ]
  },
  {
    "id": "remote-inputs",
    "suffix": "/guide/remote-inputs",
    "labels": {
      "en": "Remote inputs",
      "ko": "원격 명세와 참조"
    },
    "group": "guide",
    "targets": [
      "typescript",
      "golang"
    ]
  },
  {
    "id": "client",
    "suffix": "/guide/client",
    "labels": {
      "en": "API calls and errors",
      "ko": "API 호출과 오류"
    },
    "group": "guide",
    "targets": [
      "typescript",
      "golang"
    ]
  },
  {
    "id": "models",
    "suffix": "/guide/models",
    "labels": {
      "en": "Models and validation",
      "ko": "모델과 검증"
    },
    "group": "guide",
    "targets": [
      "typescript",
      "golang"
    ]
  },
  {
    "id": "files-links-streams",
    "suffix": "/guide/files-links-streams",
    "labels": {
      "en": "Files, Links and streams",
      "ko": "파일·Link·스트림"
    },
    "group": "guide",
    "targets": [
      "typescript",
      "golang"
    ]
  },
  {
    "id": "transport",
    "suffix": "/guide/transport",
    "labels": {
      "en": "Security and transport",
      "ko": "인증과 전송"
    },
    "group": "guide",
    "targets": [
      "typescript",
      "golang"
    ]
  },
  {
    "id": "named-clients",
    "suffix": "/guide/named-clients",
    "labels": {
      "en": "Named clients",
      "ko": "이름 있는 클라이언트"
    },
    "group": "guide",
    "targets": [
      "typescript",
      "golang"
    ]
  },
  {
    "id": "selective-client",
    "suffix": "/guide/selective-client",
    "labels": {
      "en": "Selected operations",
      "ko": "필요한 API 선택"
    },
    "group": "guide",
    "targets": [
      "typescript",
      "golang"
    ]
  },
  {
    "id": "selection-cost",
    "suffix": "/guide/selection-cost",
    "labels": {
      "en": "Selection and build cost",
      "ko": "선택과 빌드 비용"
    },
    "group": "guide",
    "targets": [
      "typescript",
      "golang"
    ]
  },
  {
    "id": "schema-vocabularies",
    "suffix": "/guide/schema-vocabularies",
    "labels": {
      "en": "Schema vocabularies",
      "ko": "스키마 어휘 확장"
    },
    "group": "guide",
    "targets": [
      "typescript",
      "golang"
    ]
  },
  {
    "id": "server",
    "suffix": "/guide/server",
    "labels": {
      "en": "Webhooks and callbacks",
      "ko": "웹훅과 콜백"
    },
    "group": "guide",
    "targets": [
      "typescript",
      "golang"
    ]
  },
  {
    "id": "troubleshooting",
    "suffix": "/guide/troubleshooting",
    "labels": {
      "en": "Troubleshooting",
      "ko": "문제 해결"
    },
    "group": "guide",
    "targets": [
      "typescript",
      "golang"
    ]
  },
  {
    "id": "examples",
    "suffix": "/examples/",
    "labels": {
      "en": "Examples",
      "ko": "예제"
    },
    "group": "examples",
    "targets": [
      "typescript",
      "golang"
    ]
  },
  {
    "id": "ai-streaming",
    "suffix": "/examples/ai-streaming",
    "labels": {
      "en": "AI streaming",
      "ko": "AI 스트리밍"
    },
    "group": "examples",
    "targets": [
      "typescript",
      "golang"
    ]
  },
  {
    "id": "report-jobs",
    "suffix": "/examples/report-jobs",
    "labels": {
      "en": "Background reports",
      "ko": "백그라운드 보고서"
    },
    "group": "examples",
    "targets": ["typescript", "golang"]
  },
  {
    "id": "webhook-server",
    "suffix": "/examples/webhook-server",
    "labels": {
      "en": "Webhook receiver",
      "ko": "웹훅 수신"
    },
    "group": "examples",
    "targets": [
      "typescript",
      "golang"
    ]
  },
  {
    "id": "reference",
    "suffix": "/reference/",
    "labels": {
      "en": "Reference",
      "ko": "참조"
    },
    "group": "reference",
    "targets": [
      "typescript",
      "golang"
    ]
  },
  {
    "id": "cli",
    "suffix": "/reference/cli",
    "labels": {
      "en": "CLI",
      "ko": "CLI"
    },
    "group": "reference",
    "targets": [
      "typescript",
      "golang"
    ]
  },
  {
    "id": "client-api",
    "suffix": "/reference/client-api",
    "labels": {
      "en": "Client API",
      "ko": "클라이언트 API"
    },
    "group": "reference",
    "targets": [
      "typescript",
      "golang"
    ]
  },
  {
    "id": "server-api",
    "suffix": "/reference/server-api",
    "labels": {
      "en": "Server API",
      "ko": "서버 API"
    },
    "group": "reference",
    "targets": [
      "typescript",
      "golang"
    ]
  },
  {
    "id": "streaming",
    "suffix": "/reference/streaming",
    "labels": {
      "en": "Streaming API",
      "ko": "스트림 API"
    },
    "group": "reference",
    "targets": [
      "typescript",
      "golang"
    ]
  },
  {
    "id": "types",
    "suffix": "/reference/types",
    "labels": {
      "en": "Types",
      "ko": "타입"
    },
    "group": "reference",
    "targets": [
      "typescript",
      "golang"
    ]
  },
  {
    "id": "capabilities",
    "suffix": "/reference/capabilities",
    "labels": {
      "en": "OpenAPI support",
      "ko": "OpenAPI 지원"
    },
    "group": "reference",
    "targets": [
      "typescript",
      "golang"
    ]
  },
  {
    "id": "compatibility",
    "suffix": "/reference/compatibility",
    "labels": {
      "en": "Compatibility",
      "ko": "호환성"
    },
    "group": "reference",
    "targets": [
      "typescript",
      "golang"
    ]
  },
  {
    "id": "extensions",
    "suffix": "/reference/extensions",
    "labels": {
      "en": "OpenAPI extensions",
      "ko": "OpenAPI 확장"
    },
    "group": "reference",
    "targets": [
      "typescript",
      "golang"
    ]
  },
  {
    "id": "playground",
    "suffix": "/playground",
    "labels": {
      "en": "Playground",
      "ko": "플레이그라운드"
    },
    "group": "playground",
    "targets": [
      "typescript",
      "golang"
    ]
  }
]);

export function sdkPath(sdk, locale, topicID = "home") {
  if (!sdkTargets.includes(sdk) || !displayLocales.includes(locale)) throw new TypeError("Unsupported documentation context");
  const topic = topics.find((item) => item.id === topicID && item.targets.includes(sdk));
  if (!topic) throw new TypeError(`Unknown documentation topic: ${topicID}`);
  return `${locale === "ko" ? "/ko" : ""}/${sdk}${topic.suffix}`;
}

// More specific prefixes must precede their containing locale.
/** @type {readonly import("./doc-contracts").DocScope[]} */
export const documentationScopes = Object.freeze([
  { key: "ko/typescript", sdk: "typescript", locale: "ko" },
  { key: "ko/golang", sdk: "golang", locale: "ko" },
  { key: "typescript", sdk: "typescript", locale: "en" },
  { key: "golang", sdk: "golang", locale: "en" },
  { key: "ko", sdk: null, locale: "ko" },
  { key: "root", sdk: null, locale: "en" },
]);
