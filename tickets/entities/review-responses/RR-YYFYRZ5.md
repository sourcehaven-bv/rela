---
id: RR-YYFYRZ5
type: review-response
title: 'Design: prune and import are not ACL-gated'
finding: rela audit prune and import change the audit log and run without an ACL permission check.
severity: minor
reason: They are operator commands that need the database credentials, the same tier as rela history-purge. Anyone with those credentials can change the tables directly. Each run is recorded as an audit-prune or audit-import row under the operator's principal, and prune enforces the 365-day floor.
status: wont-fix
---
