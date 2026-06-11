---
id: RR-9DZXKT
type: review-response
title: 'TestCacheLoggingNeverLeaksRawKey: parallel data race on global slog logger'
finding: The wave added t.Parallel() to a test that swaps slog.SetDefault; concurrent siblings' slog.Debug writes (lua/cache.go:414) race into the test's capture buffer. Reviewer reproduced reliably under -race; local just ci failed on it (cascading lua failures).
severity: critical
resolution: Removed t.Parallel() with a comment explaining the process-global constraint; verified stable across 4 runs of -race -count=4 -shuffle=on.
status: addressed
---
