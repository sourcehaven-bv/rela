---
id: RR-DYS0JA
type: review-response
title: Create audit lost on crash during implicit channel first append
finding: writeMeta runs before the append and AuditCreate last; a crash leaves meta without a create audit and later appends never write it.
severity: minor
resolution: Create audit written before meta.yaml; a crash can duplicate but not lose the record.
status: addressed
---
