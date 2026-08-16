---
id: RR-DGBVFO
type: review-response
title: Access-loss leaves a stale replica copy + re-permissioning requires re-bootstrap (accept + document, no de-perm tombstone)
finding: 'Design-review S1. Confirmed the right call is NOT to emit a per-principal de-permission tombstone: it would be indistinguishable from a real delete (replica hard-deletes authoritative data) and would make the feed a per-principal existence oracle. So a principal who loses row access keeps a stale local copy it can no longer refresh (the ''you could have made a copy'' live-world stance, consistent with sync-channel-bypasses-visible-redaction memory + CLAUDE.md). Second-order hole the plan glossed: the manifest cursor advances past now-hidden rows, so a re-permissioned principal (removed then re-added) will NOT see changes that occurred during the no-access window (cursor already past them) and holds a stale-but-looks-live copy. Not a leak; a silent-divergence correctness hole. Mitigation: any access-scope change requires the client to re-bootstrap from a full export; the cursor model cannot express ''rows you missed while you could not see them.'''
severity: minor
resolution: 'Accepted as designed (2026-08-08). No de-permission tombstone. Documented in the sync client contract: (1) access-loss retains a stale copy that can no longer refresh — accepted live-world outcome; (2) any access-scope change requires the client to re-bootstrap from a full export, since the seq-cursor cannot replay rows changed during a no-access window. Doc work folded into the implementation-phase docs-checklist.'
status: addressed
---

## Finding (design-review S1)

**Accept + document.** No de-permission tombstone. Two things to state in the
design/ticket:

1. On access-loss the replica retains its last copy but can no longer refresh it
(row stops appearing; fetch 404s). Accepted live-world outcome.
2. **Any access-scope change requires the client to re-bootstrap** from a full
export — the seq-cursor cannot replay rows that changed while the principal
couldn't see them. Without this note a re-permissioned replica silently diverges
while looking live.
