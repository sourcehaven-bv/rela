---
id: RR-DKPPGQ
type: review-response
title: Supersede read happens before the per-channel lock, so an older move can overwrite a newer value
finding: 'internal/realtime/outputs.go:393-411 (load reads r.later) runs before run() takes lockOutputs (outputs.go:450). It is called this way from service.go runInline and from outputs_runner.go:102 (load, then claim, then run). Two concurrent inline runs A (older) and B (newer) that both set status: both load, B takes the lock and writes, then A takes the lock with a stale r.later that lacks B''s done mark and writes the older value. This is exactly the in-process guarantee the plan claims. TestOutputs_SameSetSuperseded is sequential and would not catch it. Fix: take lockOutputs before load (and, in the runner, re-read the later messages after the claim under the lock), or re-read later messages inside run() under the lock just before each set.'
severity: significant
resolution: Inline and retry runs take the per-channel lockOutputs before load; TestOutputs_SupersedeReadUnderLock fails with the old order.
status: addressed
---
