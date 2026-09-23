import { defineConfig } from "vitepress";

import { buildDocsNavigation } from "./navigation.mjs";

const socialLinks = [
  {
    icon: "github",
    link: "https://github.com/jinyongp/openapi-sdkgen",
  },
];

const englishNavigation = buildDocsNavigation("en");
const koreanNavigation = buildDocsNavigation("ko");

const englishTheme = {
  socialLinks,
  ...englishNavigation,
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

const koreanTheme = {
  socialLinks,
  ...koreanNavigation,
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
    "architecture.md",
  ],
  lastUpdated: true,
  themeConfig: englishTheme,
  locales: {
    root: {
      label: "English",
      lang: "en-US",
      themeConfig: englishTheme,
    },
    ko: {
      label: "한국어",
      lang: "ko-KR",
      link: "/ko/",
      title: "openapi-sdkgen",
      description: "OpenAPI 3.x 문서에서 애플리케이션 SDK 소스를 생성.",
      themeConfig: koreanTheme,
    },
  },
});
