---
id: RR-FKVECPU
type: review-response
title: Lost commit acknowledgement and nested Tx are not covered by the rollback shortcut
finding: With TxRollsBack true the read-back is skipped, so a commit that succeeded but reported an error leaves unaudited rows; a nested Tx joins the open one.
severity: minor
resolution: Documented both cases on batchCreate.failed; CreateBatch has no caller inside an open Tx.
status: addressed
---
