---
id: RR-FNQTKK
type: review-response
title: ChannelSchema grows to five methods; fold the new fact into an existing one
finding: CLAUDE.md asks for consumer-side interfaces of one to three methods, and ChannelSchema already has four (dataentryconfig/channels.go:75-85). The plan adds a fifth because BusyFacts set the precedent. MessageProperties already looks up the same (channelType, messageType) pair. Returning a MessageFacts{Properties, CountsUnread} from it keeps the method count at four, and channelBlockErrors makes one lookup per message type instead of two.
severity: nit
resolution: ADDRESSED MessageProperties replaced by MessageType(ct, mt) (MessageFacts, bool) with Properties and CountsUnread; ChannelSchema stays at four methods and channelBlockErrors makes one lookup per type.
status: addressed
---
