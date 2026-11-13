---
id: RR-WG0GTVE
type: review-response
title: Replies inside a batch and batch channel creation
finding: replyParent reads the parent from storage, so a reply to an earlier batch message differs per backend; implicit channel creation by a batch is untested.
severity: minor
resolution: 'Plan: a reply_to pointing into the same batch is refused; conformance test for a batch that creates the channel.'
status: addressed
---
