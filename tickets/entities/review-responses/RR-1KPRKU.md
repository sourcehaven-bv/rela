---
id: RR-1KPRKU
type: review-response
title: Whitespace normalization treated fenced code blocks as insignificant
finding: semanticShape collapsed all whitespace runs in TEXT_LEAVES, which included `code`, `html` and `yaml` alongside `text` and `inlineCode`. Inside a fenced block indentation IS the content, so two blocks differing only in indentation compared as semantically equal — the write-back guard would classify a reindented Python snippet as suppressible churn and write the corruption back. The corpus measurement quoted in the docstring only ever observed the inline-code newline-folding case.
severity: significant
resolution: 'Narrowed the set to `text` and `inlineCode`, renamed to WHITESPACE_INSENSITIVE_LEAVES with the reasoning recorded. Re-ran the full 3,939-file corpus under the stricter comparison: still 0 semantic drift, confirming the loose normalization was never needed.'
status: addressed
---

## Finding

`serializerContract.ts` normalized whitespace across `{text, inlineCode, code,
html, yaml}`. The docstring justified it with "measured across the ticket
corpus, this is the only class of difference that survives a round-trip" — but
the measurement observed the `inlineCode` case and the rule was then applied to
four more node types that were never the observed problem.

Whitespace in a fenced block is not formatting. A Python snippet, a YAML
fragment and a Go fixture all mean something different when reindented, and the
comparison would have called them equal.

## Failure scenario

A serializer change reindents fenced blocks. The corpus test passes, because
`isSemanticallyEqual` cannot see the difference. `guardWriteBack` classifies it
as `churn-suppressed` on some bodies and `edited` on others, and reindented code
lands in the files.

## Resolution

`WHITESPACE_INSENSITIVE_LEAVES = {text, inlineCode}` — only the two where a
round trip legitimately re-wraps.

The important verification: re-ran the **full** corpus (3,939 files, not the
1-in-8 sample) with the stricter comparison and got 0 semantic drift. The loose
normalization was never load-bearing; it was latent risk.

Five tests pin the boundary — reindented code, YAML and HTML must compare as
different, while inline-code newline folding and paragraph re-wrapping must
still compare as equal.
