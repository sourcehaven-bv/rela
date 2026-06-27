---
id: TKT-H857QO
type: ticket
title: FormatMarkdown is not idempotent (re-formatting can change content)
kind: refactor
priority: medium
effort: s
status: done
---

## Problem

`markdown.FormatMarkdown` is not idempotent: a single goldmark render+wrap pass
can produce output that re-parses to a DIFFERENT document. E.g. `**\n*` formats
to `** *` (our `wrapParagraphs` joins the two lines), and `** *` then parses as
a thematic break `---`. The fuzzer also finds intrinsic render→reparse cases
independent of our wrap step.

This was surfaced by the sync canonical content hash (TKT-8FSBGB / FEAT-NJ9FEN):
the hash re-formats both fsstore's reflowed body and pgstore's raw body, which
must converge to identical bytes — broken if FormatMarkdown isn't a fixed point.

Discovered during crit review of #1006; the fix shipped as a follow-up PR.

## Fix

- `FormatMarkdownWithWidth` extracts a single `formatOnce` pass and iterates it
to a fixed point, so `FormatMarkdown(FormatMarkdown(x)) == FormatMarkdown(x)`
for EVERY caller (fsstore writes, data-entry rendering, the canonical hash) —
not just the hash. Convergence is ≤2 passes in practice; `maxFormatPasses` is a
backstop.
- Removed the equivalent loop from `canonical.canonicalBody`; the hash's
`body()` is now a plain `FormatMarkdown` call.
- Restored `FuzzFormatMarkdownIdempotent` as a now-TRUE invariant (passes 453k
execs; failed in seconds before the fix). `**\n*` and `0) ` pinned as seeds.

## Acceptance

- `FormatMarkdown(FormatMarkdown(x)) == FormatMarkdown(x)` for arbitrary input
(fuzz-verified).
- No behavior change for normal content; only degenerate inputs settle to a
stable form.
- All formatter consumers (store, dataentry, conflict) unchanged and green.

## Notes

Considered handling `**\n*` directly instead of iterating, but the fuzzer proved
the goldmark round-trip is intrinsically non-idempotent (a class, not one case),
so iteration to a fixed point is the complete fix. See PR #1008 + crit thread.
