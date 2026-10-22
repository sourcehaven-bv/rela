---
id: RR-OWQICK
type: review-response
title: 'A step always implies RequireOpen, which breaks when_closed: post triggers'
finding: 'systemWriter.post passes requireOpen=false for a trigger with when_closed: post (trigger.go act). If a step always implies RequireOpen, every such trigger that posts a driving message type into a closed channel gets ErrNotOpen, and round.process counts it as Skipped at debug level. That is a silent behaviour change for existing configs. Do not force RequireOpen. Add the lifecycle state the decision assumed to the expectation (e.g. SessionStep.ExpectOpen or the expected Channel.State), so a close or reopen between the head read and the append is a session conflict and gets the same re-read and retry. For a user post, keep RequireOpen as Service.Post sets it. Add a trigger_test case: when_closed: post with a driving type into a closed session channel posts and moves nothing.'
severity: significant
resolution: 'Plan updated: a session step no longer implies RequireOpen; the decider sees the lifecycle state and never moves a non-open channel, so when_closed: post still posts (TestSessionTriggerWhenClosed).'
status: addressed
---
