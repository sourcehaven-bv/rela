---
id: RR-L33RV9
type: review-response
title: Job mark runs outputs of a transition that is no longer a job transition
finding: 'internal/realtime/outputs.go:474-490 resolveMark matches a trigger job mark to the current transition by (mark.From, cause.Type, To) only and never checks Transition.job. If a reload between the trigger post and the runner round adds roles: to that transition, its outputs are compiled with the person rules (they may read {{channel.title}} and {{message.author}}) and grantChecker skips them, yet run() and identify() (outputs.go:644, :823) write them as run_as because mark.Job is immutable. A participant-chosen channel title then reaches a value written with run_as authority, which refKind documents as a security rule. Precondition: an operator config change while a trigger mark is pending; the write-time ACL still gates the row and fields. Fix: export Transition.Job() and, in resolveMark, return removed (transition_removed) when mark.Job && mark.Timer == "" and !t.Job(); add a row to TestOutputs_RunAsChanged or a sibling test.'
severity: minor
resolution: A job mark whose transition is no longer trigger-reachable fails transition_removed; tested.
status: addressed
---
