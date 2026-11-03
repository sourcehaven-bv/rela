---
id: RR-5XXIHY
type: review-response
title: Watch loses posts when an agent advances after_rev in its target
finding: 'The watch target key is a hash of the args, so a read_channel target with a new after_rev is missing from the cursor and is taken as an unreported baseline (tools_watch.go pass/result). A post that lands between read_channel and the next watch call is folded into that baseline and never wakes the agent. Fix: report a read_channel after_rev baseline with a non-empty page as changed, or spell out the baseline-then-read-then-wait loop in section 3.3 step 4, and add the race to AC7/AC11 tests.'
severity: significant
resolution: A read_channel target whose after_rev page is non-empty at baseline is reported as changed. The read, watch and read loop is spelled out in section 3.3 and the race is a case in the AC7 and AC11 tests.
status: addressed
---
