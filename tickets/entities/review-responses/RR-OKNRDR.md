---
id: RR-OKNRDR
type: review-response
title: record_current mapping drifts / reuse-of-renamed-id corrupts lineage (post-commit, cross-process)
finding: 'The record_current(entity_id→record_id) mapping is a mutable, post-commit, best-effort write outside the store tx, so it can drift from live entities: a crash between the rename''s store commit (entity.go:392-457, which atomically re-keys + tombstones oldID) and the post-commit mapping update leaves A→record stale and B unmapped, with no watermark-style reconciliation (unlike listener.go:240 catchUp). Worse (F4): rename A→B then create a NEW entity with id A — if the new-A create hook reads record_current[A] before the rename hook cleared it, new-A silently inherits B''s history (two unrelated entities merged). Fix: eliminate the mutable mapping — derive ''current record for entity_id'' from entity_versions itself (latest non-deleted version for that entity_id, tie-broken by seq), and mint a NEW record_id unconditionally on op=create (a create is a new lineage by definition; create must never consult the mapping). This removes the whole drift class.'
severity: significant
resolution: 'SUPERSEDED by round-2 finding RR-HKM0S6. The original ''resolved by derived lineage'' claim was WRONG — the sweep cannot observe a rename, so lineage cannot be derived from snapshots. Correct resolution: the mapping is still eliminated (no record_current), AND rename is captured SYNCHRONOUSLY at the choke-point as an op=rename version row carrying prev_id (old id). Lineage is walked via op=rename/prev_id rows that are recorded with ground truth, not guessed. Create always starts a fresh lineage. See RR-HKM0S6.'
status: addressed
---
