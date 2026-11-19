---
id: RR-ZHQS3I4
type: review-response
title: Kept path of a nested CreateLinked is not deferred
finding: Inside a joined transaction, a kept create ran its follow-ups at once, inside the transaction.
severity: minor
resolution: CreateLinked defers the kept follow-ups through the commit list when nested, as on success.
status: addressed
---
