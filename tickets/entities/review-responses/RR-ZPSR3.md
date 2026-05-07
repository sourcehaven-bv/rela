---
id: RR-ZPSR3
type: review-response
title: Replacement values must reject newlines to avoid breaking paragraph structure
finding: renderParagraph writes text + '\n'. A replacement value containing '\n' (or '\r') breaks the rendered paragraph silently. Validate values do not contain CR/LF; raise a Lua error otherwise.
severity: minor
resolution: Plan rejects replacement values containing \n or \r with a Lua error naming the offending key. Same constraint applied to opts.format return values in entity_refs. Covered by AC15 negative tests.
status: addressed
---

# Finding

`renderParagraph` (`markdown.go:907`) writes the `text` field followed by a
single `\n`. If a caller-supplied replacement (or a `format` callback in
`entity_refs`) returns a string containing `\n`, the rendered paragraph contains
a raw newline. Markdown parsers handle that inconsistently — either soft line
break or paragraph split. The failure mode is silent and weird.

# Resolution

Validate that replacement values contain no `\n` or `\r`. Violation raises a Lua
error referencing the offending key:

> `rela.md.resolve_refs: replacement for "TKT-1" contains a newline; use only inline markdown in replacement values`

Add a negative test.
