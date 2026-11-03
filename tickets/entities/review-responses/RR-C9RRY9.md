---
id: RR-C9RRY9
type: review-response
title: 'Docs plan misses three places that describe in: as the only source'
finding: '(1) GUIDE-realtime.md:750-754 advises "Give a type that decides a session a label: and an in:". That advice should mention session.offer as the way to avoid writing in: on every type. (2) GUIDE-mcp-server.md:529 (read_channel) and :531 (post_message) say "A message type limited by in:". Add "or by session.offer", as the plan does for the OpenAPI text (internal/openapi/channels.go:483). (3) The choices example (GUIDE-realtime.md:322) can stay, but the new paragraph should say that an authored in: can only widen a derived one (see the next finding). Add these to Documentation Impact and to the docs checklist.'
severity: minor
resolution: 'Documentation Impact now covers the advice at GUIDE-realtime.md:754, the read_channel and post_message rows of GUIDE-mcp-server.md, and the choices example. The new paragraph says an authored in: works as a union with the keyed states and that a full list goes stale when a state is added.'
status: addressed
---
