---
id: RR-Z24MW8
type: review-response
title: Tenant DSN derivation downgrades TLS and drops pool keys
finding: dsnForSchema and two sibling helpers re-serialize the DSN keeping only host/port/user/dbname/password; sslmode=verify-full becomes prefer and pool_max_conns is lost.
severity: significant
reason: Older defect outside this diff, present before BUG-JQO2PH; the listener fix does not touch dsnForSchema or its sibling helpers. Filed as BUG-BR9CXF (critical) with measure tenant-dsn-roundtrip.
status: deferred
---
