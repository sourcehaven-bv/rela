---
id: RR-XWSA1
type: review-response
title: WaitGroup.Add inside goroutine races with Wait
finding: wg.Add(1) is called inside the cron job goroutine. If c.Stop returns before Add executes, Wait returns prematurely. The wg is redundant since c.Stop returns a context that waits for running jobs.
severity: critical
resolution: Removed WaitGroup entirely, relying solely on c.Stop() context which waits for running jobs
status: addressed
---
