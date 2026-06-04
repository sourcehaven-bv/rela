---
id: RR-9LYNN
type: review-response
title: 'Watcher contract: lossy non-blocking sends, no ordering; in-process model is fine'
finding: 'Confirmed the Subscribe contract is intentionally LOSSY: full subscriber buffers drop events silently (store.go:280-281; test DropsWhenFull with bufSize=1 creates 5 entities expects 1 received). No event ordering is guaranteed. fsstore uses fire-and-forget `select { case ch<-ev: default: }`. The plan''s in-process watcher is correct, BUT the pg implementation must replicate this exact semantics: emit synchronously on the write path with non-blocking sends per subscriber, multi-subscriber fan-out, safe double-cancel, and Close() closes all subscriber channels (test CloseClosesSubscriberChannels). The ''echo'' concept in fsstore is filesystem-specific (suppressing self-writes seen by the fs notifier) and does NOT apply to pgstore — do not port it.'
severity: minor
resolution: 'Implemented in pgstore.go: per-subscriber buffered channel, non-blocking fire-and-forget emit, idempotent cancel, Close closes all channels; events emitted after commit; no fsstore-style echo tracker. Matches the lossy/unordered store.Watcher contract; watcher conformance tests pass (commit 296c5f3f). Lost-event window documented per RR-PGTUF.'
status: addressed
---

## Resolution (plan update)

- Implement `Subscribe(bufSize)` exactly like memstore/fsstore: per-subscriber buffered channel, non-blocking send (`select { case ch <- ev: default: }`), independent fan-out, idempotent cancel, `Close()` closes all channels.
- Emit events from within each write method AFTER the DB commit succeeds (so subscribers don't see uncommitted writes). For cascade delete / rename emit one event per affected entity+relation (matches `CascadeDeleteEmitsRelationEvents`).
- Do **not** port the fsstore "echo" tracker — it exists only to dedupe filesystem-notifier self-writes; pgstore has no external notifier in this scope.
- `seq`/`updated_at` columns still added now (forward-compat for future LISTEN/NOTIFY catchup), even though the in-process watcher doesn't read them yet.
