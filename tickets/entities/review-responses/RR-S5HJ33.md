---
id: RR-S5HJ33
type: review-response
title: Floating go-version 1.26 in reachability job
finding: ci.yml reachability job pinned go-version '1.26', which can resolve to an older cached patch release (BUG-219PVU).
severity: minor
resolution: 'Replaced by go-version-file: go.mod like every other job.'
status: addressed
---
