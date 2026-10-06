---
id: RR-TCRYDH
type: review-response
title: Tabs overwrote each other's drafts
finding: One localStorage key without a storage listener; the last tab to write erased the other tab's edits.
severity: significant
resolution: A storage listener loads the draft another tab wrote; a failed write never replaces the newer in-memory draft.
status: addressed
---
