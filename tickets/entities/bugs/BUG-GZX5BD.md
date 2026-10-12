---
id: BUG-GZX5BD
type: bug
title: rela.md.parse drops the closing line of multi-line HTML blocks
description: 'A parse/render round trip through rela.md left multi-line HTML comments unterminated: the line with --> disappeared. Seen in atlas (TASK-ZF91U), where scripts/recurrence.lua rewrites the task template and spawned tasks lost the --> of every multi-line comment. Fix: extractRawContent in internal/lua/markdown.go extends a *ast.HTMLBlock with HasClosure() to ClosureLine.Stop. Regression test: TestMdRoundTripHTMLComment.'
priority: medium
effort: xs
why1: extractRawContent built the raw node of an HTML block from Lines() only.
why2: goldmark keeps the closing line of a multi-line HTML block in HTMLBlock.ClosureLine, not in Lines().
why3: extractRawContent is a generic fallback for unsupported nodes and treats every node with Lines() alike; the HTML-block closure line is a goldmark detail it did not know about.
why4: The round-trip tests covered headings, paragraphs and code blocks but no HTML blocks, so the loss went unnoticed.
why5: There is no round-trip test per goldmark block kind that rela.md maps to a raw node.
prevention: TestMdRoundTripHTMLComment pins single-line and multi-line comments byte-for-byte. A new block kind handled by the raw fallback gets a round-trip case.
started: "2026-10-12"
completed: "2026-10-12"
status: done
---
