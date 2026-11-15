---
id: RR-E3SQ1AS
type: review-response
title: analyze --cleanup ignores a trigger load error
finding: 'cli/analyze.go loadTriggerRules discards the error, so with --cleanup a type only triggers reference can look unused and be removed. Fix: return the error when --cleanup is set.'
severity: minor
resolution: 'AnalyzeSchemaCmd loads the triggers itself: cleanup refuses when they do not load and the report warns that trigger references are left out. TestAnalyzeSchema_TriggersDoNotLoad pins both.'
status: addressed
---
