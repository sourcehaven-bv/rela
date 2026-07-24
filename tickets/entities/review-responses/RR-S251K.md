---
id: RR-S251K
type: review-response
title: Empty/missing-value parity between filter and predicate
finding: 'internal/filter/match.go:30-63 encodes a specific empty/missing-value contract: `prop=` (empty RHS, OpEqual) matches an empty/missing value; `prop!=` matches a present value; a missing/empty value matches NEITHER `=value` nor `!=value`. internal/predicate''s evalAttr (eval.go:117) instead returns NewNil() for a declared-but-absent field, so `entity.prop == ''x''` on a missing prop is false and `entity.prop ~= ''x''` is TRUE — the opposite of filter for the != case. When phase 2 transpiles `--where`/legacy filter strings and migrates automation `when:`/validation `Then:` onto predicate, the transpiler MUST map `prop=`/`prop!=` to the exact nil-or-empty predicate that reproduces filter''s contract, or automation/validation verdicts silently drift. This is the subtlest parity risk in the whole ticket.'
severity: significant
resolution: 'De-risked in Phase 1 by pinning the exact predicate expressions the Phase-2 transpiler must emit. internal/predicatefns/parity_missing_test.go verifies that presence-guarded predicate forms reproduce filter''s empty/missing contract (match.go:30-63 / TestMatchMissingProperty) identically across present/empty/missing bindings, including the subtle case: filter `prop!=value` maps to `entity.prop ~= nil and entity.prop ~= ''value''` (presence-guarded) NOT the raw `entity.prop ~= ''value''` (which is true on a missing field — the opposite of filter). Full mapping table documented in the test. The transpiler now has a verified target; final closure lands with the Phase-2 transpiler + golden replay of the filter corpus.'
status: addressed
---
