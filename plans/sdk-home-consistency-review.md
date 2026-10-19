# SDK home consistency

The TypeScript homes used VitePress `layout: home`, a hero, three primary actions,
four feature groups and usage links. The Go homes were plain Markdown documents,
so they rendered a sidebar and an entirely different reading layout. This was a
documentation composition mismatch unrelated to SDK capabilities.

Use the existing home layout and the same content hierarchy for both SDKs in both
locales. SDK names and scoped URLs differ; requirements and language-specific
usage remain in guides. Preserve the prior Go heading anchors. The product-level
home remains governed by its own content and navigation.

Reviewed the four source pages, shared theme/layout, rendered content and action
destinations. `devtools run build:docs` passed. Browser checks at 1280px and 390px
covered all four routes: each renders a home hero, three scoped actions, four
feature groups and the same usage-link sequence, with no page-level horizontal
overflow. Built route/link/anchor validation is recorded with the task evidence.
No new styles or components were needed.
