---
id: RR-5X1BFBE
type: review-response
title: Fold time and Go-evaluation time from one clock
finding: Lowering and evaluation could read different fallback clocks without a ctx read time.
severity: significant
resolution: No date lowering in this ticket; evaluation reads readtime.Or(ctx, evaluator clock) and list conditions use the world clock.
status: addressed
---
