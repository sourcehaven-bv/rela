---
id: RR-3UR6T2
type: review-response
title: HistoryView drops the face indicator entirely for a face with no declared label
finding: HistoryView.vue faceLabel now comes from schemaStore.faceLabel, which is '' for an unlabelled coordinate, and the v-if hides the indicator. Under a world the coordinate identifies which record is being diffed; the face NAME is operator config, not rela vocabulary, so falling back to it is defensible.
severity: minor
resolution: HistoryView falls back to the declared face NAME when no label exists; only the unlabelled bare face renders nothing, since naming it would take a rela word.
status: addressed
---
