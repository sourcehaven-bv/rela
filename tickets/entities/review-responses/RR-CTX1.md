---
id: RR-CTX1
type: review-response
title: Side effects ran inside the store transaction
severity: critical
status: addressed
finding: Version capture, alias notification and denied-write audit ran inside the Tx callback. None uses
  the tx handle. The version recorder writes on its own pool connection, so on pgstore a VersionOpDelete
  row could commit while the delete rolled back, leaving history that asserts a delete which never happened.
  audit.Filesystem.Record does blocking file I/O, which store.Transactor explicitly forbids inside fn
  because pgstore holds a deployment-wide advisory lock for the callback's duration.
resolution: deleteEntityInTx now performs store work only and returns the captured relation set; the caller
  runs every side effect after the transaction closes.
---
