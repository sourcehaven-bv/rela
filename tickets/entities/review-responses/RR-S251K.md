---
id: RR-S251K
type: review-response
title: Empty/missing-value parity between filter and predicate
finding: 'internal/filter/match.go:30-63 encodes a specific empty/missing-value contract: `prop=` (empty RHS, OpEqual) matches an empty/missing value; `prop!=` matches a present value; a missing/empty value matches NEITHER `=value` nor `!=value`. internal/predicate''s evalAttr (eval.go:117) instead returns NewNil() for a declared-but-absent field, so `entity.prop == ''x''` on a missing prop is false and `entity.prop ~= ''x''` is TRUE — the opposite of filter for the != case. When phase 2 transpiles `--where`/legacy filter strings and migrates automation `when:`/validation `Then:` onto predicate, the transpiler MUST map `prop=`/`prop!=` to the exact nil-or-empty predicate that reproduces filter''s contract, or automation/validation verdicts silently drift. This is the subtlest parity risk in the whole ticket.'
severity: significant
resolution: 'The FromFilter transpiler reproduces filter''s universal missing/empty-matches-nothing rule (match.go:39-45) via presentGuard: string fields guard `~= nil and ~= ''''`, typed fields (int/date/bool) guard `~= nil` (empty coerces to Nil there). prop=/prop!= empty forms map to the emptiness checks. Verified by the cross-engine parity test''s present-but-empty and all-missing records — caught and fixed the initial `!=value` bug where a non-nil empty string wrongly matched. Note: this closes the Phase-1 pre-verification (parity_missing_test.go) with the real transpiler.'
status: addressed
---
