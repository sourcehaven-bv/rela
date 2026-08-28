---
id: RR-6J7SO7
type: review-response
title: Spike plugin skips empty CSS assets, which the proposed build-output drift test would fail on
finding: The plan's edge-case list says a 0-byte custom.css should serve 200 and still inject ('present
  != non-empty') — the right call. But the spike plugin guards with `if (src.trim() && !src.includes('@layer
  rela'))`, so an empty emitted CSS asset is left UNLAYERED. Harmless for the cascade (empty CSS has no
  rules), but the build-output drift test proposed in the same plan — 'every emitted .css starts with
  @layer rela' — would FAIL on any empty emitted CSS asset. Minor, but it costs the implementer a confusing
  red CI run and may push them to weaken the drift test, which is the one guard against a future asset
  escaping the wrap.
severity: minor
status: addressed
resolution: wrapCss now wraps unconditionally, including empty input, so the invariant "every emitted
  stylesheet declares the layer" is literally true. Pinned by a test case for empty input and by TestBuiltCSSIsLayered
  over all 19 real assets.
---

## Recommended resolution

Either wrap unconditionally (an empty `@layer rela { }` block is valid CSS and
harmless), or have the drift test skip zero-length assets explicitly. Prefer the
former — an unconditional wrap keeps the invariant "every emitted `.css` is
layered" literally true, which is easier to test and to reason about.
