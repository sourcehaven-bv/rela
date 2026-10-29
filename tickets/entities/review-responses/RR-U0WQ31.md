---
id: RR-U0WQ31
type: review-response
title: Gate test does not go through precheckOutputs or the post path
finding: outputs_lock_test.go:188 TestSessionBranches_GateRefusesOtherEntry sets gate.active/from/to by hand, copying precheckOutputs (outputs.go:303-334). A regression in precheckOutputs, for example setting gate.to from the wrong move or leaving it unset on the early no-outputs return, would not fail the test. Nothing asserts that a gate refusal still stores the message without a notice or an outputs mark. Add one Service.Post-level case with a reminder count that changes between the pre-check and the append (an injected head or a fake store), or at least a test that drives precheckOutputs and then the decider.
severity: minor
resolution: Added TestSessionBranches_GateThroughPost, which goes through Service.Post and precheckOutputs; a reminder fires in beforeAppend, and the test asserts the gate refusal stores the post with no notice, mark or write, and that an entry without outputs still moves.
status: addressed
---
