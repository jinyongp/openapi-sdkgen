import { defineConfig, type DefaultTheme, type PageData, type LocaleConfig } from "vitepress";
import type { DocScope } from "./doc-contracts";

import { buildSDKNavigation } from "./navigation.mjs";

import { pageMetadata } from "./metadata.mjs";
import { documentationScopes } from "./docs-catalog.mjs";

const socialLinks: DefaultTheme.SocialLink[] = [
  {
    icon: "github",
    link: "https://github.com/jinyongp/openapi-sdkgen",
  },
];

const englishTheme: DefaultTheme.Config = {
  socialLinks,
  aside: false,
  search: { provider: "local" },
  outline: { level: [2, 3], label: "On this page" },
  editLink: {
    pattern: "https://github.com/jinyongp/openapi-sdkgen/edit/main/docs/:path",
    text: "Edit this page on GitHub",
  },
  footer: {
    message: "Apache License 2.0 · © Jinyong Park",
  },
};

const koreanTheme: DefaultTheme.Config = {
  socialLinks,
  aside: false,
  search: {
    provider: "local",
    options: {
      translations: {
        button: {
          buttonText: "검색",
          buttonAriaLabel: "검색",
        },
        modal: {
          displayDetails: "상세 보기",
          resetButtonTitle: "검색 초기화",
          backButtonTitle: "닫기",
          noResultsText: "결과 없음",
          footer: {
            selectText: "선택",
            selectKeyAriaLabel: "Enter",
            navigateText: "이동",
            navigateUpKeyAriaLabel: "위쪽 화살표",
            navigateDownKeyAriaLabel: "아래쪽 화살표",
            closeText: "닫기",
            closeKeyAriaLabel: "Escape",
          },
        },
      },
    },
  },
  outline: { level: [2, 3], label: "이 페이지에서" },
  editLink: {
    pattern: "https://github.com/jinyongp/openapi-sdkgen/edit/main/docs/:path",
    text: "GitHub에서 이 페이지 수정",
  },
  footer: {
    message: "Apache License 2.0 · © Jinyong Park",
  },
};

export default defineConfig({
  lang: "en-US",
  title: "openapi-sdkgen",
  description: "Generate application SDK source from OpenAPI 3.x documents.",
  base: "/openapi-sdkgen/",
  vite: {
    server: {
      host: "0.0.0.0",
    },
  },
  srcExclude: [
    "openapi-feature-inventory.md",
    "openapi-feature-matrix.md",
    "openapi-compatibility-architecture.md",
    "architecture.md",
    "_shared/**",
  ],
  transformPageData(pageData: PageData): void { pageData.frontmatter.head = [...(pageData.frontmatter.head ?? []), ...pageMetadata(pageData.relativePath)]; },
  lastUpdated: true,
  themeConfig: englishTheme,
  locales: Object.fromEntries(documentationScopes.map((scope: DocScope): [string, LocaleConfig<DefaultTheme.Config>[string]] => [scope.key, {
    label: "",
    lang: scope.locale === "ko" ? "ko-KR" : "en-US",
    link: scope.key === "root" ? "/" : `/${scope.key}/`,
    themeConfig: { ...(scope.locale === "ko" ? koreanTheme : englishTheme), ...(scope.sdk ? buildSDKNavigation(scope.locale, scope.sdk) : { nav: [], sidebar: [] }) },
  }])),
});
