---
id: RR-GV7EHG
type: review-response
title: "Deleting hiddenDisplayTitleEntityIDs regressed templated titles to partial render (tschmits PR review)"
severity: significant
status: addressed
finding: >-
  PR review (tschmits, #1243): removing hiddenDisplayTitleEntityIDs made _analyze
  issue titles for a TEMPLATED display_property render PARTIALLY when one template
  placeholder is hidden — "Jeroen" instead of falling back to the id. This is the
  BUG-R9EHKV leak class: a partial title leaks the readable half and confirms a
  hidden half exists. Gated redaction strips the VALUE (achternaam gone) but
  DisplayTitle on the redacted map still renders the readable placeholder, and
  visibility.Redact does not recompute the title (that lived in the deleted
  helper / in stripHiddenProperties, which analyze does not use). The existing
  TestACLAnalyze_RedactsHiddenTemplatedTitle missed it — it only asserted the raw
  secret was absent, not that the title fell back to the id. Confirmed unintended
  regression, not a deliberate choice.
resolution: >-
  Added `safeDisplayTitle(meta, e)` used at all 9 analyze title sites: if ANY
  property backing the display title (including one template placeholder) is
  absent from the already-redacted entity, the whole title falls back to the id —
  restoring the BUG-R9EHKV invariant, now uniform across every check. Strengthened
  TestACLAnalyze_RedactsHiddenTemplatedTitle to assert the issue Title == the id
  AND that neither the secret ("SECRET-SURNAME") nor the partial ("Jeroen")
  appears — verified to FAIL against plain DisplayTitle (got "Jeroen") and pass
  with the fix. Full internal/dataentry green under -race; lint/plimsoll clean.
---
