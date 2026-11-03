---
id: RR-V1YZ0Z
type: review-response
title: Creator-role answers are cached across channels when transitions are computed
finding: 'Approach 7.8 resets c.ops once per (anchor, channel type) group and then calls c.fireable(c.postable()) with c.head = &ch for each channel. heldRole caches a creator role in c.ops.roles whenever c.head != nil (session.go:274-287), and mayPost caches per message type in c.ops.post (authz.go:433-443). On a cardinality-many type whose transition or post roles include a creator role, channel 2 reuses channel 1''s creator answer, so `transitions` lists moves the caller cannot make, or leaves out moves they can. waitingOf avoids the cache, so `roles` and inclusion are correct while `transitions` is wrong. ListAnchor never computes transitions per channel, so this hazard is new. Fix: reset c.ops per channel; after Prime this costs no store call. Add AC 6 rows that compare each entry''s Transitions with Session(...).Transitions on two channels of one anchor created by different users.'
severity: significant
resolution: Approach 7.8 resets c.ops per channel (not per group) with c.head set; no store cost after the prime. AC 6 compares each entry's Transitions with read_session, with two channels on one anchor by different creators.
status: addressed
---
