---
id: RR-SYNCR4
type: review-response
title: 'Temp-id lost-ack double-create: prefer replica-side reconcile-before-repush over permanent server-side dedup storage'
finding: 'Push creates an entity under a temp local id and POSTs a create WITHOUT an id (remote mints it). If the create succeeds remotely but the ack (minted id) is lost before the local rename, a naive retry re-POSTs and the remote mints a SECOND entity — /api/v1 create has no idempotency token and nothing dedupes. User ruling: a permanently-stored server-side idempotency-key table is unwanted (it accumulates storage / "futzing that keeps taking up storage"); a rare double-create is acceptable if manually correctable, and a lightweight server-side add is fine only if not permanently stored.'
severity: minor
status: addressed
resolution: 'No permanent server-side dedup store was added (per user ruling). The replica creates under a temp id and, on a successful create, ADOPTS the primary-minted id via RenameEntity (recordCreate in push.go). The rare lost-ack double-create residual is accepted as manually-correctable. NOTE: reconcile-before-repush (matching a pending create against the manifest before re-POSTing) is the documented next hardening but is not yet implemented — the current behavior on a lost ack is a possible duplicate, which is the accepted bounded residual, not a regression.'
---

## Finding (design-review, fancy-browser) — with user ruling

Do NOT add a permanently-stored server-side idempotency store. Instead:

- The replica records its pending create in its OWN local push-state (state.go),
  keyed by temp id.
- On a lost ack, the replica reconciles against the manifest feed BEFORE
  re-pushing: an entity matching its pending create that now appears in the feed
  is **adopted** (local rename to the remote id) rather than re-created.
- Dedup thus lives in the replica''s transient push-state + the feed it already
  consumes — nothing permanent on the primary.
- **Accepted residual:** if reconciliation cannot confidently match a genuinely
  ambiguous case, the worst case is two items — rare, not a common occurrence,
  and manually correctable. This is a deliberate, bounded tradeoff, not a bug to
  engineer away with server storage.

## Recommended resolution

Reconcile-before-repush on the replica; no server-side persistent dedup. Document
the rare-double-create residual as accepted. If a server-side assist is ever
added, it must be transient (no permanent per-key storage).
