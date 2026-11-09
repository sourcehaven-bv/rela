---
id: RR-PP86MV9
type: review-response
title: Baseline read without settling
finding: The count-based baseline was read straight after the warm-up cycle.
severity: minor
resolution: Removed with the switch to goleak, which retries until goroutines exit.
status: addressed
---
