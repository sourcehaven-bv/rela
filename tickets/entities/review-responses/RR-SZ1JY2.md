---
id: RR-SZ1JY2
type: review-response
title: Declare layer order explicitly, and consider sub-layers before the flat layer ossifies
finding: 'Two design improvements that are cheap now and expensive later. (1) Layer ORDER is established by first encounter. With 18 runtime-appended chunks, whichever chunk loads first fixes @layer rela''s position, and a stray ''@layer foo'' declared anywhere could silently reorder against it. Emitting a bare ''@layer rela;'' declaration into the eager index.css BEFORE any content pins the order at first stylesheet parse — one line in the plugin, removing a whole class of order-dependence, which is precisely the bug class this ticket exists to kill. (2) A flat ''@layer rela'' is already showing strain on day one: RR-XOTMPN establishes that tokens.css genuinely wants different cascade treatment from component CSS, and RR-F371WB that vendor CSS does too. Sub-layers (''@layer rela.tokens, rela.vendor, rela.base, rela.components'') cost almost nothing now (declared in one statement) and give a future seam for operators to target a specific tier. Retrofitting a flat layer into sub-layers later is a BREAKING change for every operator skin that exists by then.'
severity: minor
status: addressed
resolution: >-
  Part (1) adopted: a bare `@layer rela;` declaration is emitted before any content to pin layer order at first parse. Part (2) rejected per DECISION 2: sub-layers buy nothing operator-facing, since unlayered operator CSS already beats every layer regardless of count or order.
---

## Recommended resolution

Adopt (1) unconditionally — it is one line and closes a real order-dependence
hole.

Treat (2) as a genuine design decision to settle **before** implementation
rather than after, because the migration cost is asymmetric: flat → sub-layers
is breaking for operator skins, whereas starting with sub-layers costs one
declaration.

Also worth a cheap source-side guard: a lint/unit check that no `src/**/*.css`
or `<style>` block declares a bare competing `@layer`, closing the other
direction that the build-output test cannot see.

Relates to [[rr-tokens-css-layer]] and [[rr-vendor-css-in-layer]].
