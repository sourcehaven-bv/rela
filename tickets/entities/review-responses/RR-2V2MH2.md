---
id: RR-2V2MH2
type: review-response
title: Head refetch in post bypasses onSeen and run serialization
finding: 'useChannel.ts:420 calls refetchHead directly after the first message. It does not call seen(s), so options.onSeen never receives the new head and channelSchema.checkNames cannot spot an action or message type it names. It also runs outside run(), so a frame arriving meanwhile issues a second head GET, since follow''s `unstored` is still true until this answer lands. Fix: route it through run(s, step) so run calls seen, or call seen(s) after it; extend the new test to assert onSeen received the refetched head.'
severity: minor
resolution: The first-message head refetch in useChannel.post now runs through run(), so onSeen sees the new head and a frame meanwhile waits, and the test asserts onSeen received the refetched head.
status: addressed
---
