---
id: RR-K4LKS8
type: review-response
title: PostResult.Session for a non-driving post can be stale
finding: 'internal/realtime/session.go:231 (postedSession) and service.go:336: when the posted type drives no transition, step is nil and the result is sessionView(c.head), read before the append. A concurrent move between that read and the append is not shown. Yet the MCP tool description (tools_channels.go), docs/mcp-server.md and the CLI say the answer is "the session after the post". Fix: for every log post to a session type, attach a step whose Decide returns no move when the type does not drive. The store then fills Outcome with the committed session at no extra store call. Or reword the docs to "the session as of the post".'
severity: minor
resolution: Every log post to a session type carries a step, so PostResult.Session is what the store committed under its lock; tested with a post that moves nothing.
status: addressed
---
