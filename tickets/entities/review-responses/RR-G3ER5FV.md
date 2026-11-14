---
id: RR-G3ER5FV
type: review-response
title: A 409 on a ledger group key reveals keys used by hidden owners
finding: Group keys are unique per type across all owners, so a caller who may post learns that a key is in use on owners it cannot read, and can claim a key first.
severity: significant
reason: 'Accepted and documented in GUIDE-records: a ledger transaction spans owners, so its key cannot be scoped to the caller''s owners without losing uniqueness. Operators are told to use keys that reveal nothing on their own or generated keys.'
status: wont-fix
---
