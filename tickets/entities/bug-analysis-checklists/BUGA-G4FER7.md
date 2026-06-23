---
id: BUGA-G4FER7
type: bug-analysis-checklist
title: 'Analysis: GenerateShortID can emit IDs its own validator rejects (pathological prefixes)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally — fuzz corpus input `("--", "0", 10, "0")`: `GenerateShortID` returns `"--9HHF"`, `entity.ValidateID` rejects it ("consecutive dashes not allowed")
- [x] Reproduction is deterministic (the corpus input replays it; now committed as a regression seed since the fixed oracle passes it)

## Root Cause

- [x] Five-whys completed (on the bug entity)
- [x] Root cause: no layer owned prefix validity. `GenerateShortID` treats the prefix as opaque (trims one trailing `-`, appends `-<base36>`), and `metamodel.Parse` validated prefix *declaration shape* (conflicting forms, presence) but never the *character contract* — so a prefix like `--` flowed through to generation. The generator and the validator (`entity.ValidateID`, `storeutil.ValidateID`) each enforced their half without a shared contract for the input between them.

## Fix Plan

- [x] Direction decided with reviewer (session 2026-06-12): validate at metamodel load — prefixes only enter the system via `id_prefix`/`id_prefixes`, so the load gate covers every caller, and a broken metamodel fails loudly at startup instead of producing broken IDs at write time
- [x] `metamodel.ValidateIDPrefix` (exported) + `InvalidIDPrefixError` wired into `Parse`'s hard-error path next to the existing prefix checks
- [x] `GenerateShortID` godoc documents the precondition; the fuzz oracle delegates to `ValidateIDPrefix` (no hand-modeled character rules — the same staleness class fixed in TKT-PCLGGL's harness work)
- [x] Regression seed committed (`testdata/fuzz/FuzzGenerateShortID/bug-rhfhth-double-dash-prefix`)
