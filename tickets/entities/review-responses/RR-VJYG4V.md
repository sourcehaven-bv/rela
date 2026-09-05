---
id: RR-VJYG4V
type: review-response
title: No test pins the placeholder allowlist across the Go docs and the TS implementation
finding: The four placeholder names live in worldText.ts KEYS and in the WorldMessages godoc with nothing enforcing agreement. The repo pins cross-boundary contracts by reading both files (TestAppTokensCSSInSyncWithFrontend); a placeholder added on one side renders literally with no failure.
severity: minor
resolution: metamodel.ChromePlaceholders is the Go-side allowlist; TestChromePlaceholdersInSyncWithFrontend (internal/dataentry) reads worldText.ts's KEYS line off disk and compares, the TestAppTokensCSSInSyncWithFrontend pattern.
status: addressed
---
