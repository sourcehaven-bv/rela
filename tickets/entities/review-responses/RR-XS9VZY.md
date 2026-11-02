---
id: RR-XS9VZY
type: review-response
title: Restated nil-means-every-state prose
finding: The plan adds "nil In means every state" and "config, the same for every reader" to MessageTypeDescription, MessageTypeView, ChannelMessageTypeSchema and the TS mirror. realtimecfg.MessageType.In already states the first. commentlint's duplication rule will flag the copies. Link [realtimecfg.MessageType.In] from the Go views, and add [MessageType.InDerived] to the In godoc and the InKey godoc.
severity: nit
resolution: "addressed\tGo views link realtimecfg.MessageType.In and InDerived; In and InKey godocs link InDerived; v1 holds the one wire-side sentence since it cannot import realtimecfg; the TS comment points to v1."
status: addressed
---
