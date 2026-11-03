---
id: RR-OZ03VU
type: review-response
title: checkListBusyColumn takes a *bool out-parameter
finding: busy_column.go:13 threads seen *bool through the call and sets it as a side effect. Counting busy columns in the validateLists loop (validate.go:1247), or returning the new seen value, keeps the helper pure and easier to test.
severity: nit
resolution: ADDRESSED checkListBusyColumn takes earlierBusy bool; validateLists counts busy columns and passes busyColumns > 0.
status: addressed
---
