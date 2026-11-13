---
id: RR-R1CX9EL
type: review-response
title: dataentry tests time out under -race in CI
finding: 'CI Test on PR 2244: internal/dataentry hit go test''s 10-minute default timeout (TestGanttEdges_CostIsScaleFree running). Develop already takes 500s; the two-size budget tests add about 150s.'
severity: significant
resolution: Raised the CI go test per-package timeout to 20m with a comment naming the cause.
status: addressed
---
