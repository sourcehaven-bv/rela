---
id: RR-HG6T06W
type: review-response
title: Folded compact header has two aria-current items in the markup
finding: The full list kept aria-current on its current item while CSS hid it, so one exposed current item depended on CSS; the strict expectSessionAt helper would fail on a folded compact header.
severity: minor
resolution: The full list drops aria-current while the short list is shown (contextShown). Stories now count aria-current in the markup. expectSessionAt uses :visible, and the compact e2e calls it while folded.
status: addressed
---
