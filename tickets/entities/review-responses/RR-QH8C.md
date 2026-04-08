---
id: RR-QH8C
type: review-response
title: 'F10: captureLog test helper races with non-capturing tests via global log.SetOutput'
finding: log.SetOutput is process-global mutable state. logMu only serializes tests that opt into capture, so if another test in the same package runs in parallel via t.Parallel() and emits a log line while a captureLog-using test holds the lock, the line lands in the captured buffer. Today no tests in this package call t.Parallel() so it's latent, but the moment someone adds it the KeyNeverLeaks test becomes non-deterministic. The right long-term fix is to stop using log.Printf from library code entirely and inject a logger dependency.
severity: minor
reason: 'Architectural — slog.Default() is also process-global, so just rewriting captureLog for slog (which we did during the rebase) does not actually fix the race, only renames it. The real fix is to inject a *slog.Logger into the provider via dependency injection so tests can use a per-test instance instead of swapping the global. That requires changing NewOpenAICompatProvider''s signature plus all the entry-point construction sites and is a refactor in its own right. Today no tests in the internal/ai package call t.Parallel() so the race is latent, not active. Documented as a follow-up: ''Inject slog.Logger into ai.Provider to remove global-state race in tests''.'
status: deferred
---
