---
id: RR-6N1B2F
type: review-response
title: 'Ephemeral messages: expiry, delivery and reply'
finding: '§0 and §3: "A post answered 202 adds nothing; the head refetch shows it." That depends on a live frame arriving, which fails when the stream is down or the key is past MAX_LIVE_SUBSCRIPTIONS (useLiveSubscription drops extra keys with only a console warning). Signals also stay on screen until the next frame, after `expires_at`. Fix: refetch the head directly after a 202, hide signals client-side at `expires_at` with one timer for the soonest expiry, and hide the reply-to control for ephemeral types (the service''s `Reply: !ephemeral`).'
severity: minor
resolution: After a 202 the head is refetched directly, signals are hidden at expires_at by one timer, and ephemeral types have no reply control (R17, AC 15).
status: addressed
---
