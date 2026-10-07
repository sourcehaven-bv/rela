---
id: RR-LEYSXY
type: review-response
title: Verify compares store.VersionOf across backends
finding: VersionOf hashes Go types (store/version.go:130); yaml decodes unquoted dates as time.Time and 2.0 as float64 while sqlite reads JSON back as string/int, so verify fails on ordinary data and values change form.
severity: critical
resolution: 'Plan updated: Verify with canonical.HashEntity/HashRelation after a documented value normalization; JSON-incompatible values are row errors.'
status: addressed
---
