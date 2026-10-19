# Compact shared documentation homes

The TypeScript homes used VitePress `layout: home`, a hero, three primary actions,
four feature groups and usage links. The Go homes were plain Markdown documents,
so they rendered a sidebar and an entirely different reading layout. This was a
documentation composition mismatch unrelated to SDK capabilities.

The first fix aligned SDK homes with VitePress hero/cards in `b047c98`. The user
then explicitly requested a simpler design across the product and SDK homes.
This supersedes the hero/card direction; keep the cause above as historical context.

All six EN/KO root and SDK home routes now use `SDKHome.vue`: one title, one short
description and guide/example/playground/reference links. Product homes show both
targets; SDK homes show only that target. Installation and detailed usage stay in
the starting guides. Four SDK Markdown pages retain their historical anchors and
declare `layout: page`. Removed the unused hero-specific theme styles.

Reviewed route-derived SDK state, locale handling, link destinations, headings,
focus styling and responsive layout. `devtools run build:docs`, documentation
TypeScript checks and built route/link/anchor validation passed. Browser checks
at 1280px and 390px covered all six routes: root has eight links, SDK homes have
four, each targets the correct SDK, and none has horizontal page overflow. One
shared component now owns the composition, preventing independent home templates
from drifting apart again. No new dependencies or permanent cosmetic tests.
