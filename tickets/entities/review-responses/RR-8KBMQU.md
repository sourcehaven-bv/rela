---
id: RR-8KBMQU
type: review-response
title: No postgres guard that a real pgstore yields a sweep
finding: newVersionSweep returns nil on a type miss; the new compile-time assertion only checks VersionSweep's method set, so a wrapped store would silently stop history capture.
severity: significant
resolution: 'Added versionsweep_built_test.go: builds through Discover on postgres and sqlite and requires a sweep; proven to fail on a forced type miss.'
status: addressed
---
