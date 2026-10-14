---
id: RR-IOSK6P
type: review-response
title: Non-string module error becomes a string
finding: 'run converted ae.Object to a string, so error({code=''E1''}) in a module reached pcall(require, ...) as ''table: 0x...''.'
severity: minor
resolution: Non-string error values are re-raised unchanged. Pinned by TestRequire_NonStringErrorPassesThrough.
status: addressed
---
