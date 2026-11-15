---
id: IMPL-9IFU1AG
type: implementation-checklist
title: 'Implementation: Id minting misses mixed-case ids after HighestID switch'
started: "2026-11-13"
completed: "2026-11-13"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (TestSequentialNumber)
- [x] Integration tests written (test full flow, not just units) (storetest subtests on all four backends; TestCreate_MintsPastMixedCaseIDs through the manager)
- [x] Happy path implemented
- [x] Edge cases from planning handled (dashed prefix, digits-only suffix, overflow, non-ASCII fold, retired and soft-deleted ids)
- [x] Error handling in place (errors surfaced, not swallowed) (short-id count and lookup errors fail the create, pinned by TestCreate_ShortIDFailurePaths)

## Test Quality

- [x] Using fixture builders or factories for test data (seedIDs, newIDMintManager)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end (tests reverted against the old memstore and core.go fail: minted REQ-004 instead of REQ-008)
- [x] Each acceptance criterion verified with test scenario from planning (all four backends pass the new storetest subtests; postgres run against a scratch database)
- [x] Edge cases manually verified (EXPLAIN under a generic plan: all four UNION arms use their lower() index)

**Verification Evidence:** storetest HighestIDFoldsPrefixCase and
HighestIDCountsDigitsOnly pass on fsstore, memstore, sqlitestore and pgstore.
TestFaceReadExplain/highest_id shows Bitmap Index Scans on
entities_id_lower_key, marked_entities_id_lower_idx, retired_ids_id_lower_key
and retired_ids_successor_lower_idx.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities (one shared entity.SequentialNumber replaces four parsers and ExtractHighestNumber's rule)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
