---
id: RR-FA1I
type: review-response
title: Lineage note in commit message
finding: |
  The split from TKT-IHCY7 is invisible from git log once merged. Six months from now, "why does useAutoSave have channel-disable flags?" is a question someone will ask.
severity: nit
status: addressed
resolution: |
  Single line at the bottom of the implementation commit body: "Split from TKT-IHCY7; sibling slices TKT-IHC7B (properties inline edit) and TKT-IHC7C (cards/list)." Nothing in the code itself. Captured here so the implementation step remembers to include it.
---
