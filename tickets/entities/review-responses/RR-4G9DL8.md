---
id: RR-4G9DL8
type: review-response
title: 'Operators must copy on: into in: for almost every type'
finding: 'In the PoC, 17 of the 19 message types would need an in: list that repeats exactly the states whose on: keys them (go, claim, resume, plan, approve, revise, ask, answer, permission, allow, deny, finished, fail, accept, rework, retry, abandon). Only progress and text differ. The load check guards one direction only (an on: entry outside in:). Q1 rejects deriving from on: because it would change existing files, but an opt-in switch at session level (for example session.offer: transitions, meaning "a type keyed by a transition is postable only where it is keyed unless it declares in:") would not change existing files and would remove the copying. Either add it here or file it as a follow-up and record the decision in Q1. The vocabulary choice itself fits: in: matches session.reminders.<r>.in, while lifecycle from: names channel states, not session states.'
severity: minor
resolution: 'Decided: a `session.offer: transitions` shorthand is a follow-up ticket, since it only fills `MessageType.In` at load and reuses this ticket''s runtime unchanged; the decision is recorded as Q1b and in Out of scope.'
status: addressed
---
