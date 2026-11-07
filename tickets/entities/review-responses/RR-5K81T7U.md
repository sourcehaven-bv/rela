---
id: RR-5K81T7U
type: review-response
title: Failed Start leaves the sweep pool open
finding: A failed Start leaves the one-connection pool open until Close.
severity: nit
reason: Close releases it, and every caller closes the queue; TestPostgresQueue_CloseLifecycle covers Close without Start.
status: wont-fix
---
