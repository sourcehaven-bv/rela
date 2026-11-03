---
id: AM-md-html-block-round-trip
type: automated-measure
title: 'Regression test: rela.md keeps multi-line HTML blocks intact'
description: TestMdRoundTripHTMLComment parses and renders single-line and multi-line HTML comments (including the atlas task-template checklist comment) and requires byte-for-byte equality. Catches a raw-node fallback that drops the closing line of an HTML block (BUG-GZX5BD).
kind: test
location: internal/lua/markdown_test.go (TestMdRoundTripHTMLComment)
status: active
---
