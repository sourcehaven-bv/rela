---
id: DEC-OYV2AK
type: decision
title: 'Per-deployment validation mode: advisory (default) vs strict (opt-in, blocks on error)'
context: |-
    DEC-HWZHA established that write APIs tolerate temporarily invalid data: soft conditions (required-missing, type mismatch, invalid enum, target-type mismatch, unknown meta keys) ride along as 200+warnings rather than 422, because the filesystem store is edited freely by git/editors/scripts and the API rejecting a state the filesystem tolerates is a hostile asymmetry. That reasoning is correct *for the filesystem deployment model*.

    Two newer capabilities change the calculus for a subset of deployments: (1) the postgres backend removes the filesystem hand-edit channel, and (2) ACL adds real server-side gating. For a managed/networked deployment where the data-entry write path is the mandatory chokepoint (confirmed: internal/entitymanager/CLAUDE.md mandates all writes go through Manager; Lua/MCP/automations all call Manager.{Create,Update,...}), a hard block on validation errors buys a *real* invariant rather than a false one. Such deployments want typical web-form behavior: block on error, show warning.

    The naive move — tie blocking to the postgres build tag — is wrong: it bakes a deployment assumption into a compile-time flag, and a postgres instance could still have non-UI writers. Blocking must be an explicit per-deployment policy, not a per-backend behavior.
consequences: |-
    ## The mode

    A deployment-level config `validation: advisory | strict` (default `advisory`). It shifts where entitymanager's existing hard/soft partition line sits — it does NOT introduce a new mechanism.

    - **advisory** (default): unchanged from DEC-HWZHA. Only structural-impossibility is hard (422); required-missing / type-mismatch / invalid-enum / analyze-style conditions ride as 200+warnings. Filesystem and dogfooding deployments stay here.
    - **strict** (opt-in): error-severity validation failures join the `hard` partition → the existing `newValidationError` → 422 path that structural errors already use. Warning-severity conditions still ride as 200+warnings.

    ## Severity rule (which property-level failures are 'error' in strict mode)

    Guiding principle: **if the data-entry UI's own controls make a state unreachable through normal use, that state arriving on a write is an error in strict mode** (it implies a non-UI writer or a stale client). By that rule:
    - ERROR (block in strict): required-missing, type-mismatch, invalid-enum. An out-of-enum value is impossible to produce via a `<select>`, so its presence is a defect, not an in-progress edit.
    - WARNING (ride along, both modes): the analyze-only relational conditions DEC-HWZHA enumerates — target-type mismatch, missing target, unknown/required-unset meta keys — because the UI *can* legitimately produce them mid-edit.

    ## Relationship to DEC-HWZHA

    This AMENDS, not reverses, DEC-HWZHA. advisory mode IS DEC-HWZHA verbatim and stays the default. DEC-HWZHA's caution that 'auto-save per-field feedback crept into the wire layer instead of staying a UI concern' is honored: warnings remain a UI concern in both modes; only error-severity blocking moves to the wire, and only when a deployment explicitly opts in.

    ## Mechanism already exists

    The 422 → ProblemDetail.errors[] → SPA path is how structural errors surface today (entitymanager/core.go partitionValidationErrors → newValidationError; dataentry ProblemDetail; SPA ApiError.validationErrors from PR #960). Strict mode routes more failures down the existing chute; the backend change is localized to the partition function + a config key + the property-severity semantics.

    ## Frontend is mode-agnostic and can ship first

    The SPA work (render warnings amber/non-blocking + clearing on clean save; render 422 validation errors as per-field blocking errors that prevent the green saved-state) is identical whether or not strict mode is ever built — it just renders the severity the server sends. Doing it now makes the SPA forward-compatible; the backend strict-mode work is a separate ticket on its own timeline.

    ## Out of scope

    Multi-tenant per-project mode (this is per-deployment), and making advisory deployments any stricter. ACL gating is orthogonal and already hard.
date: "2026-06-13"
status: accepted
---
