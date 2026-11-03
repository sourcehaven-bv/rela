---
id: RR-PTMKFR
type: review-response
title: The OpenAPI document is not updated
finding: 'internal/openapi/channels.go builds channelSchema by hand: anchor ... last_seq, rev. No test compares it with apiwire v1.Channel, so the new session block and its transitions would be missing from the published API with no failing test. Add the session schema (and a sentence on session notices in the message schema description) to the files to modify. This is separate from the slice-2 `session` description in the schema endpoint.'
severity: minor
resolution: 'Plan updated: internal/openapi/channels.go gets a ChannelSession schema and a session property on Channel; TestOpenAPI_ChannelSession is AC 14.'
status: addressed
---
