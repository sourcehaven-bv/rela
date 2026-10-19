---
id: RR-BJ86OG
type: review-response
title: Shutdown mid-round saves with a cancelled ctx, so a generation hand-over reposts
finding: 'internal/realtime/trigger.go:501-502 and 939-956. post() and save() use the round''s ctx. When rela-server cancels the worker for a generation change or Configure save during a round, the remaining Appends fail with ctx errors and count toward failures. save() also fails, so the posts already made are held only in t.dirty, which dies with the process. The next leader posts them again. Fix: stop posting when ctx.Err() != nil (requeue the rest), and save the touched shards with context.WithoutCancel(ctx) and a short timeout.'
severity: minor
resolution: Saves run on context.WithoutCancel of the round context with a 10 second timeout. TestTriggers_CanceledRoundSavesWhatItPosted shows that the next leader does not post again.
status: addressed
---
