---
id: RR-VSLLXC
type: review-response
title: scheduler cites memory_backend.go without naming the library
finding: 'internal/scheduler/jobs.go: the library''s memory_backend.go:160 no longer says which library.'
severity: significant
resolution: Names the job queue library that internal/jobs wraps.
status: addressed
---
