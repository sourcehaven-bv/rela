---
id: REV-XRSCWX
type: review-checklist
title: 'Review: storeutil.TopValues: hoist the triplicated property-value ranking (one copy had already drifted)'
status: done
---

<!-- @managed: claude-workflow v1 -->

PR: #1420 · branch `refactor/storeutil-topvalues-TKT-6QMDLC`.

## Automated Checks

- [x] `go test ./internal/store/...` — `fsstore` 2.9s · `memstore` 2.5s ·
`pgstore` 27.9s · `storetest` 0.8s · `storeutil` 1.0s, all ok
- [x] `go test -tags postgres ./internal/store/pgstore/` — ok, 30.4s **against
a live PostgreSQL**, so the pgstore call site is genuinely exercised rather than
skipped
- [x] `golangci-lint run ./internal/store/...` — 0 issues
- [x] `golangci-lint run --build-tags postgres ./internal/store/...` — 0 issues
- [x] `just arch-lint` — "OK - No warnings found"
- [x] Both build tags compile

## Code Review

- [x] Reviewed
- [x] Findings addressed

Self-reviewed. No agent review for this one, and that is a deliberate call
rather than a shortcut: it is an xs extraction of an existing, tested code path
with no interface or behaviour change, and the full `cranky-code-reviewer` pass
went to the two substantive tickets in this batch (TKT-415WA7, which turned up a
critical typed-nil bug, and TKT-8TJ2WN).

The one judgement worth recording is **which copy became the shared version**.
Two of the three had the allocation bug; taking pgstore's correct form was not
automatic, and it would have been easy to extract whichever copy I read first
and quietly standardize on the wrong one. `TestTopValuesAllocatesForTheResult`
now pins it so a future "simplification" back to `make([]string, 0, limit)`
fails rather than silently regressing all four backends at once.

No `review-response` entities: nothing was deferred or disputed.

## Acceptance Verification

| AC | Verdict | Evidence |
|----|---------|----------|
| 1 — one impl, three delegating sites | **PASS** | `sort` no longer imported by any of the three files |
| 2 — `limit <= 0` all; sized to result | **PASS** | `negative_limit_means_all` + `cap()` assertion |
| 3 — deterministic ranking | **PASS** | 50 runs over a 5-way tie |
| 4 — non-nil empty slice | **PASS** | nil and empty map subtests |
| 5 — conformance unchanged | **PASS** | all three suites green, pgstore on a live DB |

## Documentation

- [x] ~~User-facing docs~~ (N/A: internal refactor, no behaviour change)
- [x] Rationale documented where the next implementor will read it — the
`TopValues` doc comment states the `limit <= 0` convention, why ties break
alphabetically (Go randomizes map iteration, so without it two backends could
disagree), and names the drift that motivated the hoist
