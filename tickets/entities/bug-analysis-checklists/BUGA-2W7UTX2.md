---
id: BUGA-2W7UTX2
type: bug-analysis-checklist
title: 'Analysis: Lower-case sequential id_prefix can never mint a valid id'
started: "2026-11-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally
- [x] Minimal reproduction steps documented
- [x] Environment/conditions noted

Reproduced with `TestCreate_MintsForNonUpperCasePrefix` in
`internal/entitymanager/idprefix_case_test.go`: a sequential type with
`id_prefix: "nt-"` (and one with `"Memo-"`) fails its first create with
"entity ID NT-001 does not match any prefix for type note: [nt-]". The
validator is backend independent, so every backend is affected.

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

See why1 to why5 on the bug.

## Fix Planning

- [x] Fix approach determined
- [x] Regression test planned
- [x] Related areas checked for similar issues

Fold case in `metamodel.EntityDef.MatchesID` and make
`Validator.ValidateEntity` call it instead of its own copy of the loop.
Minting keeps writing the upper-case prefix, so the store-side numbering of
BUG-RLBXTG5 is unchanged. Related areas: `Metamodel.InferEntityType` uses
`MatchesID` and folds too. `dataentry.validateCreateIDOpts` compared a
manual id with the prefix case-sensitively when this analysis was written;
review finding RR-JE23B3A made it fold case as well, in this same change.
