---
id: RR-6Q2LH2
type: review-response
title: Security notes miss the causation channel and the real reason for the timer channel.title refusal
finding: 'The Security section (plan:496-502) argues that no poster input reaches a timer or trigger write. That is true of values, but anyone who may post the message that moves a session into a timed state, or who may create a cardinality-many channel (Service.Create, internal/realtime/service.go:589-636), or edit the entity a trigger watches, causes a run_as write they could not make themselves. The channel title is free text from the channel''s creator (service.go:627), so the real reason to refuse {{channel.title}} on timer outputs is that it would let a participant steer a run_as value, not the head-read budget given at plan:281-282 and 457-458. Plan change: put both points in the docs/acl-security.md run_as section and in the refusal comment, so that a later ''add channel.title to timers'' is judged as a security change. Also refuse {{channel.title}} on by: trigger outputs: triggers post only to cardinality-one channels, which have no title, so the reference is always empty.'
severity: minor
resolution: Causation channel and the channel.title steering reason are in Security and docs/acl-security.md; the refusal comment marks both as security rules; channel.title is refused on all job-move outputs.
status: addressed
---
