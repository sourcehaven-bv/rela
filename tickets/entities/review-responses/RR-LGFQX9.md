---
id: RR-LGFQX9
type: review-response
title: group_by on a list-valued property counted every row as unset
finding: Validation accepted a list-valued group_by, and axisKey returns no key for a list, so every row landed in one Unknown bar.
severity: significant
resolution: 'Config load rejects a list-valued group_by on a list card. Test case: list-valued group_by.'
status: addressed
---
