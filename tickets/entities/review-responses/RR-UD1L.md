---
id: RR-UD1L
type: review-response
title: 'Open question #1 (keep PropertyDisplay?) should be resolved before implementation'
finding: |
  Whether PropertyDisplay survives as a shell or gets inlined affects where the lock icon lives, where the DL chrome lives, and how cards/list factor their own chrome. It's the structural decision that gates the others. Leaving it as a post-review open question means the first PR review re-litigates the whole shape. Best answer up front: keep PropertyDisplay as the DL+lock chrome shell that owns value-cell delegation; cards/list call the registry directly because their chrome is different.
severity: nit
resolution: |
  Plan revised. PropertyDisplay.vue stays as the DL+chrome shell: its job becomes (a) render the DL layout, (b) for each value either render <InaccessibleField> (RR-UD1F) or delegate to the resolved widget in display mode. Strips the inline Badge/plain-text logic. Cards and list do NOT go through PropertyDisplay -- their chrome (.card-field row, .list-fields flat span) is intentionally different. They each inline the registry call (three lines per call site is below the "extract a helper" threshold). If TKT-HOIX1 adds a fourth call site, revisit whether to extract a helper at that point.
status: addressed
---
