# Product and SDK home boundaries

The TypeScript homes used VitePress `layout: home`, a hero, three primary actions,
four feature groups and usage links. The Go homes were plain Markdown documents,
so they rendered a sidebar and an entirely different reading layout. This was a
documentation composition mismatch unrelated to SDK capabilities.

The first fix aligned SDK homes with VitePress hero/cards in `b047c98`. The next
request asked for simpler SDK home pages. Commit `94869aa` incorrectly applied
that simplification to the product home as well. The user clarified that the
product home should retain its prior presentation; only SDK homes should be compact.

Product homes use `SDKHome.vue`, restored byte-for-byte from `b047c98`: product
introduction, overview, CLI installation and both language rows. The subsequent
request removes even the compact SDK overview: all four SDK entry URLs now use
`SDKEntry.vue` to open their own getting-started guide. The interim `SDKOverview`
component was removed. Four entry Markdown pages retain historical anchors and
`layout: page` for compatibility. SDK/locale destination construction uses the
existing context helpers, with a normal link as the unmounted fallback.

Reviewed explicit SDK props, locale handling, deployment base, native replacement
navigation and fallback links. Final `devtools run build:docs`, documentation
TypeScript and built route/link/anchor checks passed. Browser checks opened all
four entry URLs and confirmed the matching guide heading, SDK and locale after
navigation. Selecting Go from the product home's SDK selector also opened its
getting-started guide. The root product overview and installation still render;
its source component remains unchanged. No new dependencies or source-literal tests.
