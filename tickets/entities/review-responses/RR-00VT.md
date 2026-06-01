---
id: RR-00VT
type: review-response
title: initializeDefaults reads fields.value before first dry-run; fragile dependency on early-return
finding: 'initializeDefaults() iterates fields.value (the computed) before any dry-run has run. In create mode at this point stagedAffordancesReady=false so the computed early-returns allFields, which is what''s needed - but this is incidental, not intentional. Future edits to the early-return or the order of mount-time calls could silently start filtering defaults against stale affordances and miss seeding form-level defaults. Fix: in initializeDefaults, read allFields.value directly (or pass it explicitly) so the dependency on ''no affordances yet means render everything'' is local and clear, not action-at-a-distance through the computed.'
severity: significant
resolution: initializeDefaults now iterates allFields.value directly instead of the affordance-filtered fields computed. Dependency on the early-return is gone; create-mode defaults seed from every configured field regardless of any future change to fields's filter logic. Added a comment pinning the rationale.
status: addressed
---
