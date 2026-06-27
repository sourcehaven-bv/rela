---
id: RR-AE54G9
type: review-response
title: Unbounded deletions growth + cursor=0 replays all history
finding: The deletions table grows forever (every delete adds a row, nothing prunes). ManifestSince(0) replays the entire deletion history and returns the full set in one slice; on a long-lived churny dataset the manifest can dwarf the live row count. No retention horizon, no pagination.
severity: significant
resolution: 'Documented the retention caveat in the ManifestSince godoc: a fresh client should bootstrap from a full export then track the cursor, not rely on cursor 0. Tombstone pruning (retention horizon) and manifest pagination (LIMIT + next-cursor) are explicit follow-ups noted in the code, deferred from this ticket (they are their own unit of work consumed by the sync API sub-ticket TKT-PV0R3V/T4H4YK). Not silently dropped.'
status: addressed
---
