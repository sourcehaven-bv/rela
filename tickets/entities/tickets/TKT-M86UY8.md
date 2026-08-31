---
id: TKT-M86UY8
type: ticket
title: Audit rela acl who-can queries (CONTROL-8-15)
kind: enhancement
priority: medium
effort: s
status: done
---

## Description

`rela acl who-can <verb> <entity>` produces an explicit confidentiality
attestation — who can read or write a given entity, with the roles, groups,
ancestor folders and raw email addresses by which each grant was acquired — and
**nothing records that the query was run**.

GitHub issue #1145 (IB-review rela#1141), CONTROL-8-15. Severity: medium.

Notable: `FEAT-RCQ6SJ` carries a `requires → audit-log` relation in the tickets
project. That relation was never satisfied, and no review-response addressed it
— so the gap was declared in the ticket graph before it was found in review.

## Why the answer is worth logging

The output is a map of who can see what. That is exactly the reconnaissance an
attacker with shell access would want before choosing a target, and exactly the
question an investigator asks afterwards ("who was looking at the access model,
and when?").

## Minimum record

Principal, timestamp, verb and entity queried. **Not** the result set: the
answer names principals and their routes, and copying that into the audit log
would duplicate the disclosure rather than record it — the same reasoning
`OpACLBypassRead` already applies to elevated reads.
