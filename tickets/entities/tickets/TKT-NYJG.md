---
id: TKT-NYJG
type: ticket
title: Analyze page warning count out of sync with visible tables (gaps + duplicates hidden)
kind: enhancement
priority: medium
effort: s
status: done
---

## Summary

The summary badge on the data-entry Analyze page (`/analyze`) shows a warning
count (e.g. 67) that does not match the number of rows visible in the tables
below. GH issue [#785](https://github.com/sourcehaven-bv/rela/issues/785).

## Root cause (preliminary)

`runAnalysis()` in `internal/dataentry/analyze.go` returns six sections:

- Properties
- Cardinality
- Validations
- Orphans
- **Duplicates** (not rendered)
- **ID Gaps** (not rendered, and produces one warning per missing ID number)

The total `warnings` field in the JSON response sums all six sections, but the
frontend (`frontend/src/views/AnalyzeView.vue`) hard-codes `CHECK_TYPES` to only
render four: Properties, Cardinality, Validations, Orphans. So Duplicates and ID
Gaps inflate the summary count but have no visible row.

Additionally, `analyzeGaps` emits one warning per missing ID (e.g. 39 warnings
for one prefix with a long gap), which the CLI's `gaps` summary counts as 1 (per
prefix group). The number on the page reflects the per-ID count, which is what's
surfaced to the API but is invisible.

## Acceptance criteria

1. Every warning/error counted in the summary badge is visible somewhere on the page (either as a row, or in an aggregated "ID Gaps" / "Duplicates" card).
2. The number on the summary badge equals the sum of the visible per-check-card counts.
3. Existing Properties / Cardinality / Validations / Orphans behavior is unchanged.
4. The fix has frontend unit-test coverage for the new sections.

## Out of scope

- Filtering or activating/deactivating warning types (issue's "ideally" option 3).
- Reconciling CLI vs UI counting semantics for ID gaps. The page can display per-ID rows; we just have to make them visible.

## Related

- `internal/dataentry/analyze.go` — `runAnalysis()`, `analyzeGaps()`, `analyzeDuplicates()`.
- `internal/dataentry/api_v1.go` — `handleV1Analyze`.
- `frontend/src/views/AnalyzeView.vue` — `CHECK_TYPES`.
