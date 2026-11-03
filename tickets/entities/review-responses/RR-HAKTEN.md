---
id: RR-HAKTEN
type: review-response
title: System-writer post classification and the other message-only sites have no test
finding: 'internal/realtime/system.go:62-74 now marks trigger posts with postKind(appended) instead of mark, but TestLocal_MovedKeys (internal/realtime/live_subscription_test.go) has no trigger-post row. A revert to mark is harmless; a regression that drops MovedSession from appended (or a refactor that discards the return again) would leave boards stale with no failing test. AC 1 also lists a promote (promoteFailed, RunAction after promote) and an outputs progress write (outputs.go:854, 1002) as message-only; only the plain action row exists. Fix: add rows for a trigger post that moves the session (moved), a trigger post that does not (message), a failed promote (message), and an outputs progress write (message).'
severity: minor
resolution: 'Fixed: TestLocal_MovedKeys gains rows for a trigger post that moves (moved), a trigger post that does not (message), a failed promote (message) and an outputs runner retry that writes only the outputs mark (message). session_test gains harness.triggerPost, which postAsTrigger now uses.'
status: addressed
---
