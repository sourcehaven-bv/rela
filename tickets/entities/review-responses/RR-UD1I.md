---
id: RR-UD1I
type: review-response
title: FieldRenderer must pass mode="edit" explicitly, or the default is silently load-bearing
finding: |
  Risk table says "FieldRenderer doesn't pass mode (relies on default)." That makes the 'edit' default a load-bearing API choice -- if someone later flips the default to 'display' (to make views less explicit), all existing forms silently break. FieldRenderer should pass mode="edit" explicitly; the default is a fallback for tests and ad-hoc consumers, not a contract.
severity: minor
resolution: |
  Plan revised, going further than the original suggestion: dropped the prop default entirely. mode is now a required prop on WidgetProps with a strict 'display' | 'edit' typed union. FieldRenderer passes :mode="'edit'" explicitly; EntityDetail's display-mode callers pass :mode="'display'" explicitly. Existing widget tests get a mechanical search-and-replace to add mode: 'edit' to each mount call (~15 minutes). Required prop catches future typos at compile time; no silent breakage path from default-flip.
status: addressed
---
