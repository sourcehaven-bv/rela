---
id: RR-Z24MW8
type: review-response
title: Tenant DSN derivation downgrades TLS and drops pool keys
finding: dsnForSchema and two sibling helpers re-serialize the DSN keeping only host/port/user/dbname/password; sslmode=verify-full becomes prefer and pool_max_conns is lost.
severity: significant
reason: Older defect outside this diff; filed as BUG-BR9CXF (critical) with measure tenant-dsn-roundtrip.
status: deferred
---
