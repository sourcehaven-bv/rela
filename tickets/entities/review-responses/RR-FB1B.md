---
id: RR-FB1B
type: review-response
title: 'C2: _props is invented — no such wire field exists'
finding: |
  PLAN line 48 references `viewData.value.entry._props ?? properties[prop]`. The Entity type has no `_props` field. Grep across `frontend/src` confirms zero occurrences.
severity: critical
status: addressed
resolution: |
  Deleted from PLAN. The write-back is simply `viewData.value.entry.properties[p] = v` (or the spread-clone equivalent for reactivity safety — see RR-FB1G). `_props` reference removed entirely.
---
