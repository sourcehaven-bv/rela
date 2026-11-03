---
id: RR-4TFSF7D
type: review-response
title: Untokened reload moves the form but not the merge base
finding: Rows carry no _versions, so a reload that shows another writer's value leaves the base behind and the next save can report a needless conflict.
severity: minor
reason: Safe direction (no data loss) and the same as SectionEditForm. The fix is per-row _versions (TKT-MK9NJB).
status: deferred
---
