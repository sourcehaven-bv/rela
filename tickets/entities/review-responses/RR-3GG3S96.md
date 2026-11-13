---
id: RR-3GG3S96
type: review-response
title: Test gaps around deadlines, lost pins and sleeps
finding: No test for a round deadline and the next tick; LostSession did not assert the pin comes back; OnlyTheLeaderRuns proved absence with a 20ms sleep; setPerson writes past the manager without saying why.
severity: minor
resolution: 'Added TestFlowWorker_CutRoundResumes and TestDue_CreateOutlivesItsCaller; LostSession now takes a second lock on a pool with room for one; OnlyTheLeaderRuns waits for 20 more leader rounds instead of sleeping. That race run also exposed a real bug: a worker could take the lock released on shutdown and lead once with an ended context. lead now returns at once when ctx has ended. The setPerson comment names TKT-GYXOTQC. Shutdown waiting for an in-flight tick is the shared workerLife.stop, covered by the realtime worker tests.'
status: addressed
---
