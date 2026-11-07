---
id: RR-2KNCPTG
type: review-response
title: Isolation tests do not check the job queue DSN
finding: No test asserts the job queue lands in the tenant schema.
severity: nit
reason: The queue reads the same copied cfg.DatabaseURL as the store, which the decoy-DSN isolation test already pins\; the queue is covered by TKT-736J0CZ.
status: wont-fix
---
