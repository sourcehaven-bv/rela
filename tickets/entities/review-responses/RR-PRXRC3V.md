---
id: RR-PRXRC3V
type: review-response
title: Label uniqueness relied on the wall clock
finding: createLabeledRoom built the label from time.Now().UnixNano(), which can repeat or step back.
severity: minor
resolution: An atomic counter (expiryLabelSeq) makes each label unique.
status: addressed
---
