---
id: RR-39LZJBB
type: review-response
title: defer stop inside a loop in mergeRows
finding: Linters may flag defer in a loop.
severity: nit
resolution: mergeRows collects the stop functions and defers one cleanup; leastHead and mergeHead.advance split out the loop (also brings it under gocognit).
reason: The loop runs once per type and every stop must run at return; golangci-lint does not flag it in this repo.
status: addressed
---
