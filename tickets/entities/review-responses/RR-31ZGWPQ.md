---
id: RR-31ZGWPQ
type: review-response
title: Cancel detection relies on pgx error wrapping
finding: errors.Is(err, context.Canceled) works because pgx's default watcher normalizes to context.Canceled; CancelRequestContextWatcherHandler returns a PgError 57014 instead. Check r.Context().Err() as well.
severity: significant
resolution: 'Plan changed: one helper requestCanceled(r, err) checks the error chain and r.Context().Err().'
status: addressed
---
