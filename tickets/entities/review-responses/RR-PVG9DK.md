---
id: RR-PVG9DK
type: review-response
title: readWritePrep free function bypassed the directread guard
finding: readWritePrep wrapped a 3-argument GetEntityState on any store, so any dataentry file could make a direct read without an allowlist change.
severity: significant
resolution: readWritePrep is now an entityReader method; webhook_routes.go calls it on an entityReader over its raw store. The directread allowlist reason for entityreader.go names it.
status: addressed
---
