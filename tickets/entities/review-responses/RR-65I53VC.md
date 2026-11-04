---
id: RR-65I53VC
type: review-response
title: golangci-lint findings in linked.go and the batch handler tests
finding: lll on linked.go, context-as-argument on postBatch, unparam on postBatchAs.
severity: significant
resolution: Line split; ctx is the first parameter; postBatchAs always posts as alice.
status: addressed
---
