---
id: RR-CR-IMPORT
type: review-response
title: wrapCss wrapped @import/@charset, which are illegal inside @layer and silently dropped
finding: '`@import` is only valid at the top of a stylesheet and is INVALID inside a `@layer` block —
  browsers drop it entirely. `@charset` must be the literal first bytes. VERIFIED by probe: the input
  @import url("x.css");.a{color:red} produced a document where the @import is nested INSIDE the @layer
  block and would be dropped by every browser; a leading @charset was likewise preceded by the layer declaration,
  making it a no-op. Latent rather than live (no current emitted asset contains either), which is worse:
  it would land silently when a dev adds an @import for a webfont or a dependency ships @charset, and
  nobody would connect a missing font to a build plugin merged months earlier.'
severity: critical
status: addressed
resolution: 'The postcss rewrite hoists @charset/@import/@namespace above the `@layer rela;` declaration
  by construction. Pinned by two tests: @import must appear before the layer block and not inside it;
  @charset must remain the first bytes of the file.'
---

Raised by `/code-review` of the TKT-3DBK6I implementation.
