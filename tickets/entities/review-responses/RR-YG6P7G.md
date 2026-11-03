---
id: RR-YG6P7G
type: review-response
title: Seq == 1 is sound but is a proxy for a flag both stores already compute
finding: Seq == 1 is a superset of "this append created the channel". Both stores create an implicit channel only for an unnamed key with LastSeq 0 (filechannels/ops.go:218-233, sqlchannels/sql.go:634-650), so its first message is always seq 1. The only extra wake is the first post into an explicitly created named channel, which is harmless. So the rule never misses, and the plan should say why in the godoc of the helper. Two improvements. (1) Put the rule in one helper, such as `postMoved(m channels.Message) bool`, and use it at both notePosted (service.go:500-513) and systemWriter.post (system.go:62-76). The plan describes the rule twice, and system.go already has sp.step.Outcome.Moved at line 75. (2) Optionally surface the `created` flag that appendTarget computes and discards (filechannels/ops.go:39, sqlchannels/sql.go:636) as a never-stored field on the returned Message, as ReplyTarget does (channels.go:393-399), so that the intent is stated rather than inferred. Pin with a channelstest case that an implicit create returns Seq 1.
severity: minor
resolution: 'Adopted the created flag: Message.CreatedChannel is a never-stored field both stores set from appendTarget, pinned by a channelstest case, and one postMoved helper used by notePosted and systemWriter.post. Seq == 1 is recorded as rejected alternative 8.'
status: addressed
---
