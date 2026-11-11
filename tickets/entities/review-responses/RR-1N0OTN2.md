---
id: RR-1N0OTN2
type: review-response
title: A failed ref write after a create is neither counted nor repaired
finding: The issue was raised but Pushed.Failed was not counted and nothing set the ref later, weakening the lost-ledger safeguard.
severity: significant
resolution: setRef counts the failure, and pushEntity sets a missing ref on a linked row. TestPush_RefIsRepaired.
status: addressed
---
