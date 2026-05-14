---
id: RR-TSDB
type: review-response
title: Goldmark parser concurrency not asserted — add a race-detector test
finding: internal/dataentry/mentions.go declares `mentionsMarkdown` as a package-level singleton and pulls Parser() from it inside scanCodeSpanCandidates. Multiple concurrent HTTP requests against /api/v1/_views/... call collectMentions and therefore share the same goldmark.Markdown / parser.Parser instance. Reading goldmark's parser.go (v1.8.2 lines 864–890), Parser.Parse() uses `sync.Once` for initialization and creates fresh ParseConfig / ast.Document per call — it LOOKS thread-safe and the rest of dataentry/helpers.go already shares mdConverter the same way, so this pattern is in use. But goldmark's docs don't make a formal guarantee, and a regression in goldmark would be hard to detect. Add a unit test that fires N goroutines through scanCodeSpanCandidates with overlapping inputs and runs with -race, so the race detector catches any future change in goldmark internals. Cheap insurance against a debugging nightmare.
severity: minor
resolution: Added TestCollectMentions_ConcurrentScanIsSafe -- 64 goroutines scan against the shared mentionsMarkdown instance and assert identical results. goldmark.Markdown is configuration-only so concurrent use is safe; the test pins this invariant.
status: addressed
---
