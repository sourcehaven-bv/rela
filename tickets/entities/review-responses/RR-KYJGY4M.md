---
id: RR-KYJGY4M
type: review-response
title: Totals reload does not wait for other rows' debounced edits
finding: autoSaveBusy sees only the content autosave, so a reload can land while another row's edit is in its debounce.
severity: minor
reason: No value is lost (held fields are skipped) and a second reload follows that row's save. Same behaviour as list rows today; counting row writes in holdsWrites is a separate change.
status: deferred
---
