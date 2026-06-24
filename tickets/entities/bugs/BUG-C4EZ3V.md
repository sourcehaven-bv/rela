---
id: BUG-C4EZ3V
type: bug
title: 'FormatMarkdown non-idempotent: goldmark-markdown pads code-span whitespace unboundedly; renderer also panics on link-ref-defs'
description: '`markdown.FormatMarkdown` is not idempotent on bodies with an inline code span whose content ends in whitespace, in certain multi-line contexts: the goldmark-markdown renderer (v0.5.1, latest) pads such a span by one space per render pass (6→14→22→… spaces), so `formatOnce` never reaches a fixed point and the #1008 8-pass cap returns a still-growing result. This breaks the canonical content hash''s cross-backend convergence (fsstore reflowed body vs pgstore raw body canonicalize to different values), which surfaced as 9 of 4426 ticket bodies re-pushing on every fresh sync pull. Separately, the same renderer panics (nil deref) on a bare link-reference-definition like `[x]:0`, which formatOnce did not recover from — hostile input could crash the process. Fix: formatOnce recovers from renderer panics (falls back to unformatted input); a fence-aware `normalizeCodeSpanWhitespace` collapses runaway padding so all representations of the orbit converge to one canonical value, restoring idempotency and the cross-backend hash.'
priority: high
why1: 'FormatMarkdown(FormatMarkdown(x)) != FormatMarkdown(x) for some bodies: the goldmark-markdown renderer adds one space inside an inline code span on each render pass, growing without bound, so the fixed-point loop never converges and returns a different value each call.'
why2: 'The #1008 fix (TKT-H857QO) iterates formatOnce to a fixed point with an 8-pass cap, assuming a fixed point exists; for code spans whose content ends in whitespace (near nested-backtick spans and reflowed prose) no fixed point exists, and the cap silently returns a half-grown result.'
why3: The renderer (teekennedy/goldmark-markdown v0.5.1, latest) re-emits code-span trailing whitespace with extra padding to 'protect' it, and that padding compounds on re-parse. Separately, the same renderer nil-derefs (panics) on a bare link-reference-definition like '[x]:0', which formatOnce did not recover from.
why4: The idempotency contract was enforced only by a bounded fixed-point loop plus a fuzzer, but the fuzzer's random corpus never generated the specific multi-line code-span-with-trailing-whitespace structure that triggers the renderer's runaway padding, so the gap shipped undetected. The bound was treated as a 'pathological backstop' rather than a correctness boundary that must hold.
why5: 'We depend on a third-party markdown renderer (goldmark-markdown) for canonicalization but treated its output as trustworthy/total: there was no normalization layer asserting our OWN invariants (code-span content is literal; rendering never crashes) on top of it, and no real-corpus idempotency check. Convergence was assumed to exist rather than guaranteed by construction.'
prevention: FuzzFormatMarkdownIdempotent now pins both failing inputs as seed corpus; formatOnce recovers from renderer panics; normalizeCodeSpanWhitespace collapses runaway padding so all representations converge to one canonical value (also fixing the cross-backend hash for sync).
status: done
---

## Discovery

Found during the FEAT-NJ9FEN sync e2e: 9 of 4426 ticket bodies re-pushed on
every fresh pull because the server-stored `H(fmt(x))` differed from the
client's re-hash `H(fmt(fmt(x)))`.

## Root cause

`internal/markdown` `FormatMarkdown` is not idempotent on bodies containing an
inline code span whose content ends in whitespace, in certain multi-line
contexts. The `goldmark-markdown` renderer pads such a span by one space per
render pass (6→14→22→… spaces), so `formatOnce` never reaches a fixed point and
the 8-pass cap returns a still-growing result. This broke the canonical content
hash's cross-backend convergence (fsstore reflowed body vs pgstore raw body
canonicalize to different values).

Separately, the same renderer panics (nil deref, renderer.go:105) on a bare
link-reference-definition such as `[x]:0` — pre-existing on develop;
`formatOnce`'s `md.Convert` had no panic recovery, so hostile input could crash
the process.

## Fix

- `formatOnce`: `defer`/`recover` → fall back to the unformatted input on a renderer panic (same degrade as a Convert error).
- `normalizeCodeSpanWhitespace`: collapse 2+ spaces before a backtick run to one, applied per prose line within `wrapParagraphs` (fence-aware, so fenced code content is preserved verbatim). Removes the only growing dimension → restores idempotency AND makes every representation of the orbit (raw / once-reflowed / N-times-reflowed) map to one canonical value, fixing the cross-backend hash.
- Regression tests: idempotency on the real body, cross-representation convergence, fence-safety, panic recovery, helper unit cases; both fuzz failures saved as seed corpus.

## Verification

`rela fmt` is now idempotent on the real repro (TKT-2RCP);
FuzzFormatMarkdownIdempotent clean over 3.16M execs; canonical + fsstore +
dataentry + conflict + templating suites pass.

Follow-up to #1008 / TKT-H857QO. Affects FEAT-NJ9FEN (removes the 9-record sync
oscillation).
