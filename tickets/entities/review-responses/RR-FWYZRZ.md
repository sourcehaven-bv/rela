---
id: RR-FWYZRZ
type: review-response
title: 'Generation-wide `superseded` drops set: writes of unrelated properties'
finding: 'Decision 6 skips any `set:` when the session generation is past the notice generation, "one Head read before the set", and the rule is not limited to retries. Take two posts: A (contained->assessed, set risk) and B (assessed->notified, set status). If B''s move commits before A''s set runs, A''s risk is never written, although nothing else writes risk. This happens even in one process, because the order in which posts take the per-channel in-process lock is not their append order. Plan change: supersede per property. Skip only the properties that a later mark of the same channel names in its set:, and write the rest. Or apply `superseded` only to retries (attempt > 1), not to the first attempt. Fix the TestRunner_SetSuperseded wording and add TestOutputs_DisjointSetsBothWritten.'
severity: significant
resolution: 'Supersede is per property: before a set, skip only properties a later mark''s done set wrote, under the channel lock. Added TestOutputs_DisjointSetsBothWritten and TestOutputs_SameSetSuperseded.'
status: addressed
---
