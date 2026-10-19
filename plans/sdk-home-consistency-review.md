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
introduction, overview, CLI installation and both language rows. SDK homes use
`SDKOverview.vue`: one title, one short description and four scoped links. Both
targets/locales share that compact SDK component, with an explicit typed SDK prop.
Four SDK Markdown pages retain their historical anchors and `layout: page`.

Reviewed explicit SDK props, locale handling, link destinations, headings,
focus styling and responsive layout. `devtools run build:docs`, documentation
TypeScript checks and built route/link/anchor validation passed. Browser checks
at 1280px and 390px covered all six routes: product homes have introduction and
installation sections plus eight document links; SDK homes have neither section
and retain four correctly scoped links. None has horizontal page overflow.
Separating product and SDK presentation prevents changes to SDK simplicity from
also replacing the product overview. No new dependencies or cosmetic tests.
