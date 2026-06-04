---
id: RR-UD1F
type: review-response
title: Lock-icon ownership unresolved; "short-circuit at the call site" triplicates it
finding: |
  Today PropertyDisplay owns the lock affordance with inaccessibleTooltip (incl. git-crypt special case). Open question #6 leans toward "short-circuit at the display mode caller." That means cards (EntityDetail 639-651) and list (668-676) each grow their own lock branch -- three copies of the same affordance. Triplicating non-trivial tooltip logic is the refactor anti-pattern the ticket claims to be avoiding.
severity: significant
resolution: |
  Plan revised. Extracted into new frontend/src/components/common/InaccessibleField.vue -- a tiny component owning the lock icon, tooltip, reason branching (including the git-crypt special case). All three display modes use the same pattern: <InaccessibleField v-if="field.inaccessible" :reason="field.inaccessibleReason" /> short-circuit before the widget delegation. Three one-line consumers, single owner of the affordance. Cards/list now showing the lock where they didn't before is documented as deliberate behaviour delta #5 and #6 in the ticket -- justified as a fix (users get an explanation for missing values).
status: addressed
---
