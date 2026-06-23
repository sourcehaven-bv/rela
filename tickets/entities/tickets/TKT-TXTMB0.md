---
id: TKT-TXTMB0
type: ticket
title: 'Backend: validation: advisory|strict config that blocks on error-severity (DEC-OYV2AK)'
kind: enhancement
priority: medium
effort: m
status: backlog
---

Backend half of FEAT-13863O / DEC-OYV2AK. Opt-in per-deployment `validation:
advisory|strict` (default advisory = today's DEC-HWZHA behavior verbatim).

**Mechanism (localized)**
- entitymanager/core.go already does `hard, soft := partitionValidationErrors(errs); if len(hard) > 0 { return newValidationError(hard) }; warnings = soft`. Strict mode moves error-severity property failures into the `hard` partition so they hit the existing 422 → ProblemDetail.errors[] path that structural errors already use. No new mechanism.
- New deployment config key `validation: advisory|strict`. NOT tied to the postgres build tag (a postgres instance can still have non-UI writers; blocking is a deployment policy, not a backend behavior).

**Property-severity semantics (the real decision, per DEC-OYV2AK)**
- Rule: a state the data-entry UI's controls can't produce through normal use is ERROR in strict mode.
- ERROR (block in strict): required-missing, type-mismatch, invalid-enum (a `<select>` can't emit out-of-enum).
- WARNING (ride along, both modes): the analyze-only relational conditions DEC-HWZHA enumerates (target-type mismatch, missing target, unknown/required-unset meta keys).
- Note: write-path checks (Meta.ValidateEntity) don't currently carry a severity tag the way custom rules do — this ticket adds/derives that classification.

**Out of scope:** the frontend rendering (separate A7 ticket, ships first and is
mode-agnostic); per-project mode; touching advisory deployments. Blocked-by the
frontend ticket only in the sense that shipping strict without the UI rendering
422s would be a poor experience — but they're independently mergeable.
