---
id: RR-22MMUX
type: review-response
title: Third copy of flock code
finding: 'Architecture design review: flock exists in sqlitedb and kvpiles.'
severity: minor
resolution: 'Plan: move kvpiles.FileLocker to internal/filelock, used by kvpiles and kvlease.'
status: addressed
---
