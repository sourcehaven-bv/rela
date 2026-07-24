---
id: RR-I3VLJV
type: review-response
title: Global commands run with RELA_ENTITY_ID absent (not empty) — unset-variable expansion hazard in copied scripts
finding: |-
    buildGlobalInput (commands.go:219-224) sets only Context and Project:

      func (h *commandHandler) buildGlobalInput() *commandInput {
          return &commandInput{
              Context: "global",
              Project: h.projectInfo(),
          }
      }

    In buildCommandEnv (commands.go:669-691) the entity env vars are appended only when input.Entity != nil:

      if input.Entity != nil {
          env = append(env,
              "RELA_ENTITY_ID="+input.Entity.ID,
              "RELA_ENTITY_TYPE="+input.Entity.Type,
          )
      }

    So for context:global these variables are ABSENT from the environment, not set-to-empty.

    FAILURE SCENARIO: an admin copies an existing entity-command script as the starting point for a new dashboard button (the natural workflow once dashboard commands become possible — which is exactly what this ticket enables). The script contains something like `rm -rf "$RELA_PROJECT_ROOT/$RELA_ENTITY_ID"` or `rela delete "$RELA_ENTITY_ID"`. Under `sh -c` an unset variable expands to the empty string with no error, yielding `rm -rf "$RELA_PROJECT_ROOT/"`.

    This is a documentation and test gap, not a code defect. docs/data-entry.md is already in the plan's scope.

    ACTIONS:
    1. Document in docs/data-entry.md that context:global scripts receive NO RELA_ENTITY_ID / RELA_ENTITY_TYPE, and recommend `set -u` in command scripts.
    2. Add a Go test asserting these vars are ABSENT (not empty) from buildCommandEnv output for a global-context input — pins the contract.
severity: minor
resolution: Addressed in PLAN-CNDJ78 in two places. Documentation Planning now requires docs/data-entry.md to state that context:global scripts receive NO RELA_ENTITY_ID / RELA_ENTITY_TYPE (absent, not empty — buildGlobalInput at commands.go:219 sets no Entity, so the append at commands.go:676 is skipped), with a `set -u` recommendation for command scripts. The Test Plan adds a Go acceptance criterion asserting the vars are absent rather than empty in buildCommandEnv output for a global-context input, pinning the contract against future regression.
status: addressed
---

Low severity because it requires operator error to trigger, but the consequence
is destructive and the ticket is what makes the workflow reachable. Cheap to
close: one doc paragraph, one assertion.
