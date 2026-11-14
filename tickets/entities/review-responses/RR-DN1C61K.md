---
id: RR-DN1C61K
type: review-response
title: New ledger state without a version bump
finding: An older binary refuses a ledger holding a deleting row.
severity: minor
reason: Refusing is the safe failure for a downgrade; bumping the version would refuse every ledger instead of only those with deletes.
status: deferred
---
