---
id: RR-DIS7ZA
type: review-response
title: The runner has no defined discriminator for job marks vs person marks
finding: 'Approach 8 (plan:384-391) says Writer is display-only and job marks use session.RunAs(), and Approach 9 (plan:396-398) moves closeOnly to a current-config test, but nothing says how the runner decides that a mark is a job mark. Trigger marks have Cause > 0 like person marks. If the runner infers this from the transition shape (by: trigger, or trigger-reachable with no roles), a person mark on a trigger-reachable transition (AC 12) that is retried by the runner fails run_as_changed. The runner always retries: r.retry is always true (internal/realtime/outputs_runner.go:135) and req.Retry is always set (outputs.go:681). If it infers it from the Writer prefix, an integration:/system: poster (a CLI --run-as or a connector posting via MCP) is misread. managerOutputs.as also checks retry before anything else (internal/appbuild/realtime_outputs.go:71), so a new RunAs branch placed after it never runs. Plan change: classify from immutable mark facts only, either an explicit immutable OutputMark field (e.g. Job bool, added to the CheckOutputStep immutable list) or `mark.Timer != "" || cause.Author.User == principal.UserRealtime`. RunAs takes precedence over Retry in managerOutputs.as. Add a test where a person mark on a trigger-reachable transition fails inline with a retryable error and the runner retry writes as the poster.'
severity: significant
resolution: OutputMark gains immutable Job flag set at mark creation; runner and writer branch on it only; managerOutputs.as checks RunAs before Retry; AC 13 tests a person retry on a trigger-reachable transition.
status: addressed
---
