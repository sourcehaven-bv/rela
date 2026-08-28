---
id: RR-7YXR83
type: review-response
title: Lua misreports arbitrary hard errors as "entity not found" via strings.Contains
finding: 'luaUpdateEntity (runtime.go:1754) detected a missing entity with strings.Contains(err.Error(), "entity not found") over the WHOLE error text. Several non-404 hard errors embed caller-supplied values: internal/statemachine/enforce.go:94 formats ''%w: %s %q→%q is not a declared transition'' with the value the caller supplied. Reviewer confirmed with a runnable repro against the real manager — error text ''illegal transition: status "in-review"→"entity not found" is not a declared transition'' contains the magic substring. FAILURE: a script calls rela.update_entity("TASK-1", {status = "entity not found"}) on an entity that EXISTS; the write is correctly refused as an illegal transition, but the script sees ''entity not found: TASK-1'' and may take a compensating action such as recreating a row that is still there. mapTransitionError output and ACL denial text are the same class of hazard. The comment''s premise was also wrong: PatchEntity returns fmt.Errorf("%w: %s", ErrEntityNotFound, id), so the sentinel DOES survive errors.Is — internal/cli/update.go:83 already uses the correct idiom in this same changeset.'
severity: significant
resolution: |-
    Replaced text matching with a STRUCTURAL marker. entitymanager gained an unexported entityNotFoundError type exposing EntityNotFound() bool and Unwrap() to ErrEntityNotFound; lua declares a consumer-side NotFoundError interface plus isEntityNotFound() using errors.As. PatchEntity returns newEntityNotFound(id).

    errors.Is(err, ErrEntityNotFound) keeps working via Unwrap, so the 14 existing sentinel users are unaffected. Chose the interface over moving the sentinel into internal/entity (would have touched 14 files across many packages) and over duplicating the sentinel in lua — I verified two separately-constructed errors.New values with identical text do NOT match under errors.Is, so that approach would have silently never fired.

    Regression test TestUpdateEntity_NonNotFoundErrorIsNotMisreported reproduces the reviewer's exact scenario and was verified to FAIL against the old string-matching implementation and PASS against the fix. TestUpdateEntity_NotFoundStillReported pins that a genuine 404 keeps its original script-facing message.
status: addressed
---
