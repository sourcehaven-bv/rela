---
id: RR-Y30XRLM
type: review-response
title: 'Test gaps: rejected-file audit, wiring, hidden field, busy slot'
finding: No test for the denied-write audit of a rejected file, SetAttachFiler in dataentrywire, a visible:-hidden file property, or the busy upload slot.
severity: minor
reason: Rejected-file audit test added. The other paths are the shared attachmentWriteCheck/storeUpload already covered by the upload and attach_render tests; dataentrywire has no wiring test for attach_render either.
status: deferred
---
