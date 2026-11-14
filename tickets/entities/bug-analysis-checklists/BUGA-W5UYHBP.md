---
id: BUGA-W5UYHBP
type: bug-analysis-checklist
title: 'Analysis: Id minting misses mixed-case ids after HighestID switch'
started: "2026-11-13"
completed: "2026-11-13"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (TestCreate_MintsPastMixedCaseIDs minted REQ-004 instead of REQ-008 before the fix; storetest HighestIDFoldsPrefixCase failed on memstore)
- [x] Minimal reproduction steps documented (store holds Req-001, REQ-003, rEq-007; create a requirement)
- [x] Environment/conditions noted (fsstore, memstore, pgstore; sqlitestore already folded case but parsed differently)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined (shared entity.SequentialNumber; pgstore range on lower(id) served by the existing lower() indexes)
- [x] Regression test planned (storetest subtests on all backends, entitymanager create test, pg EXPLAIN on entities_id_lower_key)
- [x] Related areas checked for similar issues (short-id minting; found a separate defect: a lower-case sequential id_prefix never validates, filed separately)
