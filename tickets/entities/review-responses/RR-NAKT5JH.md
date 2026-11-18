---
id: RR-NAKT5JH
type: review-response
title: Every audited write takes the pg write lock
finding: All writes now open a transaction, which on pg takes the advisory write lock.
severity: minor
resolution: Documented in docs/audit-log.md Delivery.
status: addressed
---

All writes now open a transaction, which on pg takes the advisory write lock.
