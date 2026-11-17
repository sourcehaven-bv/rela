---
id: RR-T9QTKB9
type: review-response
title: Rows of one batch listed in random order
finding: All rows of a batch share one timestamp, so ids differed only in random bytes.
severity: minor
resolution: newID adds the row's place in its batch. TestAppend_BatchKeepsOrder.
status: addressed
---
