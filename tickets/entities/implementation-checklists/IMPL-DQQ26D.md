---
id: IMPL-DQQ26D
type: implementation-checklist
title: 'Implementation: GenerateShortID can emit IDs its own validator rejects (pathological prefixes)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written (TestValidateIDPrefix table; TestParse_InvalidIDPrefixRejected + _Plural + _ManualIDType loader tests)
- [x] Integration — loader gate exercised through Parse; fuzz target re-run 45s clean with the new oracle
- [x] Happy path implemented (metamodel.ValidateIDPrefix + InvalidIDPrefixError wired into Parse's hard-error path)
- [x] Edge cases handled (id_prefixes plural form; manual id_type scope pinned deliberately; trailing-dash normalization)
- [x] Error handling (typed error, errors.As-able, actionable message)

## Test Quality

- [x] Fuzz oracle delegates to the exported production contract — no hand-modeled character rules (the TKT-PCLGGL staleness class)
- [x] Regression seed committed (testdata/fuzz/FuzzGenerateShortID/bug-rhfhth-double-dash-prefix) — passes under the fixed contract
- [x] Reviewer brute-forced the completeness invariant: all prefixes ≤4 chars over a superset alphabet — zero gate-accepted prefixes whose generated IDs fail ValidateID

## Manual Verification

- [x] Repro confirmed fixed: 45s fuzz of FuzzGenerateShortID clean (was failing in 2s)
- [x] All 6 shipped/prototype metamodels still load (TestLoad_AllShippedMetamodels)
- [x] Sequential-ID path verified covered (same GetIDPrefixes gate; digit suffixes can't create "--")
- [x] just ci green; arch-lint clean (entity→metamodel is test-only, no cycle)

## Quality

- [x] Follows package patterns (typed errors, hard-error path placement)
- [x] No security issues; no silent failures; no debug code
