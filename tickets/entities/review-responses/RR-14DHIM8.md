---
id: RR-14DHIM8
type: review-response
title: Reassembly-refusal test only runs on fs and sqlite
finding: The guard is build-agnostic but its test was in a tagged file.
severity: minor
resolution: Moved to sharedbase_test.go (untagged), with a new empty-DSN test.
status: addressed
---
