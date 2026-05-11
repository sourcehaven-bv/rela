---
id: RR-ZK8N
type: review-response
title: collectMentions swallows non-NotFound errors silently — context cancellation should propagate
finding: 'internal/dataentry/mentions.go lines 41-48: any error from `s.GetEntity` other than ErrNotFound is silently dropped with a comment claiming ''transient I/O'' degradation is acceptable. Two problems. (1) No log line means a misconfigured store, a corrupted file, or a permissions issue manifests only as missing links — the SPA shows code spans where users expected pretty links, and operators have nothing to grep. At least log at warn/debug with the ID and error. (2) `context.Canceled` and `context.DeadlineExceeded` also get swallowed, so a cancelled request still iterates through every candidate doing useless work. Add an `if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) { return nil }` early-exit, or just check ctx.Err() at the top of each loop iteration. The intent (degrade gracefully on missing entities) is right; the implementation is too coarse.'
severity: significant
resolution: Non-ErrNotFound errors are logged via slog.WarnContext and skipped. Context cancellation honored via ctx.Err() check. New tests TestCollectMentions_StoreErrorIsLoggedAndSkipped and TestCollectMentions_ContextCancellationStops.
status: addressed
---
