---
id: RR-AUF5ZC
type: review-response
title: 'Store-level state invariants unfrozen: headless states and cross-state type consistency'
finding: 'The frozen contract covers Get/Put/Delete/rename/query but not two invariants §2.1 implies ("states are not independent entities... share the type"): (a) may a non-default state row exist when the default row does not (headless state)? (b) must Type match across one id''s state rows? The compound PK permits both. Options: store enforces (rejects headless create / type mismatch) vs store stays dumb (storage truth; upper layers — the Step-3 copy kernel, validation — enforce; godoc marks the conditions; storetest pins the permissive behavior). Needs the architect''s call BEFORE PR-A freezes the contract; the cascade semantics (delete-by-id sweeps all states) work either way.'
severity: significant
resolution: 'Architect decided 2026-08-20 (design doc §6 amended): the STORE enforces both — Put rejects a headless state (non-default row when no default row exists) and a type mismatch within one id''s state family; both are metamodel-free storage facts, and enforcing at the store keeps every direct writer (sync, future data migrations) honest at one choke point. The fsstore LOAD path tolerates both when found on disk (hand-edited layouts must not brick a project) and surfaces them via analyze findings (same family as undeclared-pointer detection, PR-C); cascades handle headless families defensively. Folded into the plan''s PR-A (write-path rejection + storetest cases) and PR-C (analyze findings) work lists.'
status: addressed
---
