---
id: RR-XXN6OK0
type: review-response
title: fsstore keeps an alias for the store sentinel
finding: errBodyConflictBlock was only an alias of store.ErrConflictBlock.
severity: nit
resolution: fsstore returns store.ErrConflictBlock directly.
status: addressed
---
