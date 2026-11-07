---
id: IMPL-PEA6ZJP
type: implementation-checklist
title: 'Implementation: Reconciliation element and reader view hash'
started: "2026-11-07"
completed: "2026-11-07"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

New leaf package `internal/reconcile` (Element, Kind, ParseKind, EntityElement,
RelationElement, EntityAddress, Fingerprint with Add/Sub/Limbs,
FingerprintFromLimbSums, Range with Add/Remove/Merge), 100% covered.
`canonical.Digest`, `SumEntity`, `SumRelation`; `HashEntity`/`HashRelation` are
their hex form (values pinned by goldens and checked equal to develop). Entity
and relation ETags go through `viewETag(canonical.Sum*(view), rawBody, edges)`.
Integration: the ETag tests drive the GET/PATCH and relation-read HTTP handlers
(`etag_redaction_test.go`), and the golden-parity wire tests were regenerated
(only ETag header lines changed).

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

Hardcoded values are only the deliberate goldens (fingerprint and canonical hash
encodings), which exist to fail on any encoding change.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**
- Ran `rela-server` on a copy of `prototypes/data-entry/project` as
alice@example.com. GET /api/v1/tickets/TKT-001 returned ETag "YxOm9LDA+bM=". A
PATCH with If-Match "stale" returned 412. A PATCH of the body with the current
ETag returned 200 and ETag "DWSekm7QFXQ=", and the next GET returned the same
ETag. (The GET does not answer 304 here because leases are wired; that is
existing behaviour.)
- GET /api/v1/tickets/TKT-001/relations/blocks/TKT-002 returned the quoted
ETag "ELFSOPeCPYc=" and a repeat with If-None-Match returned 304.
- AC1: `TestRelationETag_IsViewHash`, `TestEntityETag_HiddenEqualsAbsent`,
existing CAS/world/face ETag tests pass.
- AC2: `TestElement_RedactedFieldDoesNotChangeViewHash`,
`TestEntityETag_ExcludesHiddenFields`, `TestRelationETag_ExcludesHiddenMeta`,
`TestEntityETag_DropsRedactedNames`.
- AC3: `TestFingerprint_Golden` (independently recomputed in Python),
`TestHash_Golden`.
- AC4: `TestRange`, `TestFingerprint_Arithmetic`, `TestLimbs_RoundTrip`,
`TestFingerprintFromLimbSums_MatchesAdd`.
- Formatting-only body edit: `TestEntityETag_FormattingOnlyBodyEditMovesIt`.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

golangci-lint (changed packages), `just arch-lint` and `just comment-lint` are
clean. One shared `viewETag` helper serves both ETags.
