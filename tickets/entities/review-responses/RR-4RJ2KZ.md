---
id: RR-4RJ2KZ
type: review-response
title: Soft-delete reopen subtests are still named as if they took different index paths
finding: 'internal/store/fsstore/softdelete_test.go:59 and :138: the cases are "clean close" and "rescan", but both now rescan. The only difference left is whether Close ran. Rename "rescan" to "no close" so the table axis reads correctly. The table is still worth keeping, because Close still unwires echoes and closes subscribers.'
severity: nit
resolution: Renamed the "rescan" subtests to "no close" in both soft-delete tables.
status: addressed
---
