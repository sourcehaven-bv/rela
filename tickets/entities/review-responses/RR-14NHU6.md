---
id: RR-14NHU6
type: review-response
title: tickFor attributes post-execution clock times and collapses multiple firings
finding: tickFor records *clock after runDueTasks returns and assigns that one timestamp to every call observed in the batch, so N tasks firing in one tick are indistinguishable from one task firing N times. It only works because the test double is instantaneous; against a real doExecuteTask (which advances the clock between start and completion) the asserted ladder offsets would no longer describe when the task actually started. Capture the clock before runDueTasks and have the double record its own start.
severity: minor
resolution: mockTracker gained recordAt/getTimes; the double now records the run's START time from inside the run, and tickFor returns those recorded times instead of sampling the clock after runDueTasks. Multiple firings in one tick are now distinguishable, and the ladder offsets stay meaningful against an execution that advances the clock.
status: addressed
---
