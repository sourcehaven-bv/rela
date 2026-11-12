---
id: RR-0QSRC19
type: review-response
title: A run cut off by the round deadline is not backed off and blocks the tick
finding: When advance failed on the round deadline, DueRound backed off with the expired context, so the save failed and the run kept its place at the front of the due order. runTick then broke and the next tick started at the front again, so one slow run starved every other due run.
severity: significant
resolution: 'DueRound backs off on a detached context bounded by backoffTimeout (10s). On a failed round runTick keeps rep.Next, which points at the cut run, so the next tick goes on after it. Tests: TestDue_CreateOutlivesItsCaller asserts the backoff after the context ended; TestFlowWorker_CutRoundResumes asserts the resume.'
status: addressed
---
