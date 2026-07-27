---
id: RR-UUIP74
type: review-response
title: Analyze title fallback missed templated display_property (leaked hidden placeholder)
severity: critical
status: addressed
finding: >-
  Code review of the analyze surface fix found that `hiddenPrimaryEntityIDs`
  decided title-fallback using `EntityDef.GetPrimaryProperty()`, which returns
  "" for a TEMPLATED `display_property` (e.g. `{voornaam} {achternaam}`). For a
  templated type the function hit `continue` and never flagged the entity, so
  `visibleAnalysisIssues` kept the title analyze had baked from RAW properties —
  leaking a hidden placeholder value. Failure: `persoon` with
  `display_property: "{voornaam} {achternaam}"` and `achternaam` hidden by
  `visible:`; a readable persoon that trips any analyze check emits
  `title: "Jeroen SECRET-SURNAME"` to a viewer who cannot read `achternaam`.
  Same bug class, same ticket — missed because the check was built on the single
  writable primary rather than the full display read-set. The mentions and
  settings surfaces were NOT affected: they recompute `DisplayTitle` on the
  redacted entity, so a template re-renders with the hidden slot stripped.
resolution: >-
  Renamed to `hiddenDisplayTitleEntityIDs` and switched to
  `EntityDef.DisplayProperties()` (the full read set — bare primary OR every
  template placeholder). The entity is flagged if redaction strips ANY display
  property; the whole title then falls back to the id (a partial render like
  "Jeroen " would leak the readable half and confirm a hidden half, so full
  fallback is correct). Pinned by `TestACLAnalyze_RedactsHiddenTemplatedTitle`,
  verified to fail against the old `GetPrimaryProperty` logic (leaked
  `Jeroen SECRET-SURNAME`) and pass after.
---
