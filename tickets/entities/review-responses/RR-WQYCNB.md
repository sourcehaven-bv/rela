---
id: RR-WQYCNB
type: review-response
title: ChannelReader/ChannelWriter exceed the 1-3 method consumer-interface rule
finding: tools_channels.go:21-37 declares 4 methods each. The watch path uses only Subscribe (newChannelWake), so a one-method waker interface for watch and a reader for the tools would match the house rule.
severity: nit
resolution: Split into ChannelReader (Enabled, ListAnchor, Read), ChannelWriter (Post, Edit, Delete), ChannelCreator (Create) and the one-method ChannelWaker (Subscribe) for watch.
status: addressed
---
