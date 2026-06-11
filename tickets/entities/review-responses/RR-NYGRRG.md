---
id: RR-NYGRRG
type: review-response
title: catchUpInterval is an unsynchronized package global read by listener goroutines (data race)
finding: 'cranky #3: catchUpInterval is a plain package var (listener.go:27). SetCatchUpIntervalForTest (export_test.go) writes it from a test goroutine while every live listener goroutine reads it (listener.go context.WithTimeout(ctx, catchUpInterval)). Two such tests interleaving (or a listener from test A still running during test B''s mutation/cleanup) is a textbook data race that -race will eventually flag intermittently. CLAUDE.md mandates race detector on in CI with no opt-out. Hasn''t tripped yet (tests sequential, timing), but it''s latent and must be fixed.'
severity: significant
resolution: 'Fixed: catchUpInterval is now an atomic.Int64 (nanoseconds), set in init() and via SetCatchUpIntervalForTest.Store()/cleanup.Store(); the listener reads time.Duration(catchUpInterval.Load()). No unsynchronized global. Verified race-clean across repeated -race runs.'
status: addressed
---

## Resolution

Make catchUpInterval an atomic.Int64 (nanoseconds); the listener reads via
time.Duration(catchUpInterval.Load()) and the test hook + its cleanup Store()
it. Removes the unsynchronized global read/write entirely.
