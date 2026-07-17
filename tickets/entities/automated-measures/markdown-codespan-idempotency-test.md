---
id: markdown-codespan-idempotency-test
type: automated-measure
title: 'Test: FormatMarkdown idempotent on code-span whitespace + recovers from renderer panics'
description: 'Regression tests pinning BUG-C4EZ3V: FormatMarkdown idempotency on the runaway code-span-whitespace body, cross-representation canonical convergence, fence-content preservation, renderer panic recovery, and the restored FuzzFormatMarkdownIdempotent invariant with both failing inputs as seed corpus.'
kind: test
location: internal/markdown/idempotency_codespan_test.go, internal/markdown/fence_safety_test.go, internal/markdown/panic_recovery_test.go, internal/markdown/parser_fuzz_test.go
status: active
---

Regression tests in `internal/markdown` pinning BUG-C4EZ3V:

- `TestFormatMarkdown_CodeSpanGrowthIsIdempotent` — FormatMarkdown(FormatMarkdown(x)) == FormatMarkdown(x) on the real runaway-whitespace body (testdata/idempotency_codespan_growth.md).
- `TestFormatMarkdown_CodeSpanGrowthConvergesCrossRepresentation` — raw body and a once-reflowed body canonicalize to the same value (the cross-backend hash property sync depends on).
- `TestNormalizeCodeSpanWhitespace` — helper unit cases (collapse runaway padding before a backtick run; preserve a single space; leave non-span spacing alone).
- `TestFormatMarkdown_FencedBacktickContentPreserved` — fenced code-block backtick content with internal spaces survives verbatim.
- `TestFormatMarkdown_RecoversFromRendererPanic` — the goldmark renderer nil-deref on a link-reference-definition is recovered, not propagated.
- `FuzzFormatMarkdownIdempotent` — both failing inputs saved as seed corpus; clean over 3.16M execs.
