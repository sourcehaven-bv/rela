---
id: RR-KZF1WJ
type: review-response
title: golangci-lint analysis cache not persisted
finding: Four runs re-analyse the module on a cold runner.
severity: minor
resolution: Added an actions/cache entry for ~/.cache/golangci-lint keyed on go.sum and .golangci.yml.
status: addressed
---
