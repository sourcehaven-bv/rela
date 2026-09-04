---
id: RR-U3T6HQ
type: review-response
title: Unauthenticated callers hold process-wide writeMu across a full type scan
finding: '[security] runPipeline (webhook_routes.go:202-203) takes h.write.writeMu — the single process-wide mutation mutex — and holds it across the ENTIRE pipeline: up to webhookMaxAttempts=4 passes, each doing a full ListEntities(Type) scan (:309), a raw GetEntity body re-read (:450), two goldmark parses per append_section step, plus the manager write. webhookExecTimeout=30s bounds one delivery but not queueing. MEASURED: with 3000 seeded entities carrying 48KB bodies, one delivery held writeMu for 38ms; ~26 req/s saturates the mutex outright, blocking every SPA save, action and sync write. TKT-X06LA2 was literally ''fix the writeMu DoS'' on the sibling action surface, and its fix (move the authz gate before writeMu.Lock) does NOT transfer here because the endpoint is unauthenticated by design — there is no gate to move. Strictly worse than the ticketed action-surface bug, which at least required reaching an authenticated surface. Note the lock is acquired BEFORE the write that would be denied, so even a hook whose principal has zero grants still costs a full scan and a lock hold per request. Fix: (1) a shared bounded semaphore in front of runPipeline returning 429 when full — same shape as cmdexec''s pool, and like it built once, not per request; (2) do findTarget''s scan OUTSIDE writeMu, since the conflict-retry loop already provides the correctness the lock was buying there. Also reconsider 30s as a held-write-mutex ceiling.'
severity: significant
resolution: 'SPLIT: the DoS half is fixed here; the lock half is deferred to TKT-34XS2R. (1) DoS — fixed. webhookRouter gained a bounded admission channel (webhookMaxInFlight = 8), built once at router construction like cmdexec''s pool. A delivery that cannot get a slot is SHED with 503 + Retry-After before touching writeMu, rather than queued behind it; a buffered channel with a non-blocking send gives reject-when-full directly. Covered by TestWebhookRoutes_ShedsLoadWhenSaturated (asserts both that a saturated router sheds AND that freeing a slot lets the next delivery through, so the bound cannot latch) and TestWebhookRoutes_BusyIsRetryable (503 + Retry-After, not a 4xx a producer would treat as permanent). (2) Lock — deferred, with the investigation recorded: writeMu is genuinely REQUIRED today. Removing it fails TestWebhookConflict_PipelineAppendsAllLand on every run, because Patch.Content is an absolute replacement computed from an earlier read and nothing below is a compare-and-swap. The review''s other suggestion (move findTarget outside the lock) is safe but does not help — the retry loop re-finds inside the lock anyway. The real fix is store-level optimistic concurrency (TKT-34XS2R), which also removes the identical latent TOCTOU under the data-entry API''s If-Match. runPipeline''s doc comment and docs/webhooks.md both now state the limitation and name the follow-up instead of implying the conditional writes already cover it.'
status: addressed
---

[security] See `finding`. The measurement is the important part: this is not a
theoretical DoS. A single delivery against a realistic store (3000 entities,
48KB bodies) held the process-wide write mutex for **38ms**, so roughly 26
requests/second from an unauthenticated caller saturates it and stalls every
other writer in the process.

Two properties make it worse than the sibling bug that already has a ticket
(TKT-X06LA2, "fix the writeMu DoS"):

1. That fix moved the authorization gate *before* `writeMu.Lock()`. It cannot
transfer here — the endpoint is unauthenticated by design, so there is no gate
to move.
2. The lock is taken *before* the write that would be denied, so a hook whose
principal has **zero grants** still costs a full type scan and a lock hold per
request.

Fix has two independent halves, both cheap:

- A shared bounded semaphore in front of `runPipeline`, 429 when full. Same
shape as `internal/cmdexec`'s bounded pool — and, like that pool, it must be
built **once** and shared, or the cap bounds nothing.
- Move `findTarget`'s scan outside `writeMu`. It is a read, and the
conflict-retry loop already supplies the correctness the lock was providing
there — that is the whole premise of the conditional-write design.
