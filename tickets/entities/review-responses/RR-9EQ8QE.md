---
id: RR-9EQ8QE
type: review-response
title: InRule is an exported string coupled to In, and nothing enforces that they match
finding: 'The plan adds MessageType.InRule string: empty when In is nil, otherwise the key path. Two exported fields must agree, and nothing makes them. A MessageType built by hand with In and no InRule (one exists at internal/realtime/usertext_test.go:82) gives a 409 with an empty detail, without any error. The plan also drops the shared rule construction that session.go:484 and trigger.go:1246 build today. Change: store the source rather than the path, for example an unexported inDerived bool set only by the loader, plus a method (ct *ChannelType) InKey(mt *MessageType) string. It returns ct.messageTypeKey(mt)+".in" by default and <session key>.offer when derived. The zero value then gives today''s string, both runtime sites call one helper, and there is no field to keep in step. If you keep the exported field, make it unexported with an accessor so only the loader sets it.'
severity: minor
resolution: Replaced InRule with an unexported derived flag set only by the loader and one helper, ChannelType.InKey(mt), whose zero value gives today's message_types.<mt>.in path. Both runtime sites call it, and AC 4 pins the hand-built MessageType case.
status: addressed
---
