---
id: RR-XH1T6CQ
type: review-response
title: A 2xx create with an unusable answer is not held
finding: A create answer that does not decode, names another type, has an invalid id, names an already linked row, or has a 2xx other than 200/201 counted as failed and was sent again next run.
severity: significant
resolution: All these cases go through uncertainCreate and are held. Covered by TestPush_CreateMayHaveReachedTheRemote.
status: addressed
---
