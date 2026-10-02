const standaloneRoutes = ["/"];

const navItems = [
  { route: "/guide/getting-started", labels: { en: "Start", ko: "시작하기" } },
  { route: "/guide/generate", labels: { en: "Guides", ko: "사용 가이드" } },
  { route: "/examples/", labels: { en: "Examples", ko: "예제" } },
  { route: "/reference/", labels: { en: "Reference", ko: "레퍼런스" } },
  { route: "/playground", labels: { en: "Playground", ko: "플레이그라운드" } },
];

const sidebarGroups = [
  {
    prefix: "/guide/",
    labels: { en: "Build and use your SDK", ko: "SDK 만들기와 사용하기" },
    items: [
      {
        route: "/guide/getting-started",
        labels: { en: "Create your first SDK", ko: "첫 SDK 만들기" },
      },
      {
        route: "/guide/inspect",
        labels: { en: "Find APIs", ko: "생성할 API 찾기" },
      },
      {
        route: "/guide/generate",
        labels: { en: "Generate and verify", ko: "SDK 생성과 검증" },
      },
      {
        route: "/guide/client",
        labels: { en: "Call your API", ko: "클라이언트로 API 호출" },
      },
      {
        route: "/guide/named-clients",
        labels: { en: "Generate clients for each feature", ko: "기능별 클라이언트 생성" },
      },
      {
        route: "/guide/selective-client",
        labels: { en: "Load selected operations", ko: "필요한 API만 불러오기" },
      },
      {
        route: "/guide/transport",
        labels: { en: "Authentication, transport, and streams", ko: "인증·전송·스트림" },
      },
      {
        route: "/guide/server",
        labels: { en: "Receive webhooks and callbacks", ko: "웹훅과 콜백 수신" },
      },
      {
        route: "/guide/schema-vocabularies",
        labels: {
          en: "Custom JSON Schema vocabularies",
          ko: "사용자 정의 JSON Schema 어휘",
        },
      },
    ],
  },
  {
    prefix: "/examples/",
    labels: { en: "Examples", ko: "예제" },
    items: [
      { route: "/examples/", labels: { en: "Overview", ko: "개요" } },
      {
        route: "/examples/ai-streaming",
        labels: { en: "AI streaming API", ko: "AI 스트리밍 API" },
      },
      {
        route: "/examples/webhook-server",
        labels: { en: "Webhook receiver", ko: "웹훅 수신" },
      },
    ],
  },
  {
    prefix: "/reference/",
    labels: { en: "Reference", ko: "레퍼런스" },
    items: [
      { route: "/reference/", labels: { en: "Overview", ko: "개요" } },
      { route: "/reference/cli", labels: { en: "CLI", ko: "CLI" } },
      {
        route: "/reference/client-api",
        labels: { en: "Generated client API", ko: "생성된 클라이언트 API" },
      },
      {
        route: "/reference/server-api",
        labels: { en: "Generated server API", ko: "생성된 서버 API" },
      },
      {
        route: "/reference/streaming",
        labels: { en: "Streaming API", ko: "스트리밍 API" },
      },
      {
        route: "/reference/typescript-types",
        labels: { en: "TypeScript types", ko: "TypeScript 타입" },
      },
      {
        route: "/reference/capabilities",
        labels: { en: "OpenAPI support", ko: "OpenAPI 지원 범위" },
      },
      {
        route: "/reference/compatibility",
        labels: { en: "Compatibility results", ko: "호환성 검증 결과" },
      },
      {
        route: "/reference/extensions",
        labels: { en: "OpenAPI x-* extensions", ko: "OpenAPI 확장" },
      },
    ],
  },
];

export const localeParityExceptions = Object.freeze([]);

export const publicDocRoutes = Object.freeze(
  [...new Set([
    ...standaloneRoutes,
    ...navItems.map((item) => item.route),
    ...sidebarGroups.flatMap((group) => group.items.map((item) => item.route)),
  ])].sort(),
);

export function buildDocsNavigation(locale) {
  if (locale !== "en" && locale !== "ko") {
    throw new TypeError(`unsupported documentation locale: ${locale}`);
  }
  return {
    nav: navItems.map((item) => ({
      text: item.labels[locale],
      link: localizedRoute(item.route, locale),
    })),
    sidebar: Object.fromEntries(
      sidebarGroups.map((group) => [
        localizedRoute(group.prefix, locale),
        [
          {
            text: group.labels[locale],
            items: group.items.map((item) => ({
              text: item.labels[locale],
              link: localizedRoute(item.route, locale),
            })),
          },
        ],
      ]),
    ),
  };
}

export function docsNavigationLinks(locale) {
  const navigation = buildDocsNavigation(locale);
  return [
    ...navigation.nav.map((item) => item.link),
    ...Object.values(navigation.sidebar).flatMap((groups) =>
      groups.flatMap((group) => group.items.map((item) => item.link)),
    ),
  ];
}

export function localizedRoute(route, locale) {
  if (locale === "en") return route;
  if (route === "/") return "/ko/";
  return `/ko${route}`;
}
