---
id: RR-FRC2GD
type: review-response
title: 'The guard tests asserted the rule EXISTS, never that it takes effect — and missed a live offender'
finding: 'All seven guards are `readFileSync` + regex over source text. They can confirm a declaration is present; none can see whether it wins the cascade. The test file''s own comment argued grep is the only assertion that "can actually fail for the right reason" — but the failure mode it cited (TKT-CBSTYLE shipping a specificity bug under a green test) is precisely the failure mode that recurred in RR-FRC1SP, and grep did not catch it either. Separately, the ring guard matched the literal `box-shadow: 0 0 0 2px` plus two specific rgba triples, so it enforced "don''t reintroduce THESE colours at THIS width" rather than the ticket''s actual invariant. It missed `DocumentsPanel.vue:252` — `box-shadow: 0 0 0 3px rgba(59, 130, 246, 0.1)`, a hardcoded BLUE at 3px, wrong on both axes the regex keyed on, and the identical defect.'
severity: significant
resolution: 'Generalised the ring guard to match any spread-only `box-shadow: 0 0 0 <n>px` at any width, in any colour notation (hex/rgb/rgba/hsl/hsla), and to follow a declaration across line wraps; it now requires a custom property. Swept `DocumentsPanel.vue` onto the shared tokens. Mutation-verified: restoring the 3px blue ring fails the guard with file, line and offending text — the narrow version passed it.'
status: addressed
---

The carve-outs the reviewer flagged as correct are kept and unchanged: comment
lines are skipped, `var(--x, rgba(...))` dead fallbacks are skipped (they
cannot render), and `utils/palette.ts`'s legitimate `#6366f1` is pinned by a
negative test so a future "cleanup" cannot sweep the default palette's accent.

**What is still not covered, stated plainly rather than papered over.** These
guards read source, so they cannot assert cascade behaviour. The reviewer's
suggestion — parse the BUILT css with postcss and assert that each
forced-colors rule's specificity ≥ the `outline: none` it must beat — is the
assertion that would have caught RR-FRC1SP mechanically. Not built here: it
needs a specificity model over the emitted bundle, which is a meaningful piece
of tooling rather than a test tweak. Recorded as a follow-up on the ticket so
the gap is visible instead of implied.
