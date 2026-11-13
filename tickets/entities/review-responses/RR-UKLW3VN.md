---
id: RR-UKLW3VN
type: review-response
title: If-Match checked once outside the Tx for a position move
finding: A relation PATCH that only moves position checks If-Match outside the transaction.
severity: nit
reason: Meta changes are re-checked inside the Tx through ExpectedVersion. Position is not synced so the window does not affect sync; follow up with the position API.
status: deferred
---
