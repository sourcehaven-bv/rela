---
id: RR-D0L7L0
type: review-response
title: Sweep advisory lock could leak to a pooled connection on shutdown
finding: 'cranky-code-reviewer #3: tick() deferred advisoryUnlock with the tick''s ctx; when Close cancels it mid-tick, the unlock Exec fails on the cancelled ctx, the session-scoped advisory lock is not released, and (pgxpool doesn''t reset session state on Release) rides the pooled connection back into the pool — locking out other processes from sweeping until that connection is recycled. Plus #4: WriteVersion ran schema-ensure + version-insert as two non-transactional statements (crash between them loses the delete version). Plus #8: RenderProjection.JSON panicked, on the write path.'
severity: significant
resolution: '#3: unlock now runs on context.WithoutCancel(ctx) so it executes during shutdown; the closed-conn case is recognized and not warned. #4: WriteVersion wraps both inserts in one tx (all-or-nothing). #8: JSON() returns ([]byte,error); the version hook logs+skips rather than panicking, honoring ''versioning must never fail a write''. Also #7 (make_interval instead of Duration.String()::interval avoids the sub-ms micro-sign parse failure), #9 (removed dead scan var), #6 (restore maps TOCTOU races to a clear retry message), and an architect nit (silent sweep no-op now logs).'
status: addressed
---
