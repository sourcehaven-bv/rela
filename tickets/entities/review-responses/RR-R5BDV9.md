---
id: RR-R5BDV9
type: review-response
title: Typing POST in flight can land after the log post and resurrect the entry for 6s
finding: 'frontend/src/components/entity/channels/OpenChannel.vue:221 calls typing.reset() after the post, but useTyping.ts:42-55 does not wait for or cancel an in-flight sendTyping. When a refresh fires on the last keystroke before Enter, the two requests run concurrently; if the typing call reaches typingStore.touch after Post''s clear (internal/realtime/service.go:353), the poster shows as ''is typing...'' for a full TTL after the message appeared, with an extra start frame. Fix (client): in onPost, await the pending typing promise (expose it from useTyping) before ch.post, and make reset() also mark a generation so a late resolve does not re-arm. Optionally (server) refuse a touch whose c.now precedes the identity''s last log post in that channel.'
severity: minor
resolution: useTyping.settle() blocks new sends and awaits a pending one before ch.post; reset() in finally; two tests in useTyping.test.ts.
status: addressed
---
