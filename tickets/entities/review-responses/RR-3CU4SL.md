---
id: RR-3CU4SL
type: review-response
title: No test checks goldmark segments render correctly in marked
finding: Hand-written segments in vitest and no marked in Go tests let S3 and S4 through.
severity: significant
resolution: TestSegmentsGolden writes internal/comments/testdata/segments_golden.json (16 shapes incl. setext, closing hashes, link refdefs, HTML blocks, nested lists, tables, backslash, task items, starts inside emphasis/link); commentHighlight.golden.test.ts renders each through marked+DOMPurify and checks the marks.
status: addressed
---
