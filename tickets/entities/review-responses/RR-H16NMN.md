---
id: RR-H16NMN
type: review-response
title: Marker read-modify-write race between gate adoption and migration runner
finding: state.KV has no compare-and-swap; a gate evaluation racing a migration run (multi-process pg) can overwrite the runner's marker with a stale Applied list, so a later `migrate data` could re-run an already-applied file. Steps are idempotent, but the old rename_property both-keys branch mutated data on re-run.
severity: significant
resolution: 'Three-part response (commit bddc13f3): (1) the data-loss amplifier is gone — rename_property now touches NOTHING on a both-keys conflict (surfaced as a note; re-runs converge), so a re-applied file is a true no-op; (2) Resolve already skips a file whose `to` shape is the current hash even when the Applied entry was lost, and every step is idempotent, so the race degrades to harmless re-planning; (3) the last-write-wins semantics and the recovery reasoning are now documented on SaveMarker, with a cross-process lock named as the upgrade if multi-writer deployments routinely migrate under live traffic. Migrating under live multi-writer traffic is already operationally discouraged in docs/data-migration.md.'
status: addressed
---
