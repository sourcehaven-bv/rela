---
id: RR-97VOON
type: review-response
title: 'Self-echo nonce: no per-store ID exists; Notification.PID is useless for self-id'
finding: 'Design-review verification: the plan''s self-echo filter needs a per-store nonce, but (1) the Store struct (pgstore.go:61-70) has NO instance ID and New() (102-114) never assigns one; (2) Notification.PID is the LISTENING backend''s pid, not the writer''s, so it can''t identify our own writes. The nonce must be generated per-store in New/Open and embedded in EVERY pg_notify payload; the listener compares payload.nonce == our nonce to skip self. Without this, a process double-emits its own writes (once from the immediate in-process emit, once from the round-tripped notification).'
severity: significant
resolution: 'Implemented as planned: added a per-store originID (8 random bytes from crypto/rand, hex-encoded) generated in New() and stored on the Store. The producer embeds it as the first SEP-joined field of every pg_notify payload; the listener''s handleNotification skips any notification whose origin == s.originID (our own write, already emitted in-process). Remote writes carry a different origin and ARE emitted by the listener. Verified by TestSelfNotificationFiltered (unit test on handleNotification) and TestCrossProcessPropagation (remote origin delivered).'
status: addressed
---

## Resolution (plan update)

Add a per-store `originID` (random, generated in New/Open via crypto/rand or a
uuid-like string) stored on the Store. Producer includes it in the payload:
`<origin>:<kind>:<op>:<id>`. Listener skips any notification whose origin ==
s.originID (that's our own write, already emitted in-process). Remote writes
have a different origin and ARE emitted by the listener.

Note: the immediate in-process emit stays (local writes are instant + listener-
independent, RR-... R1 from the plan). The listener emits ONLY for remote
origins. This keeps local correctness decoupled from the listener being up.

Edge: a process restart changes its originID — harmless (it just won't recognize
its OWN pre-restart notifications, but those are already delivered/gone).
Catch-up de-dups via idempotent re-snapshot regardless.
