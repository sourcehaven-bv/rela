---
id: RR-J188VJ
type: review-response
title: Continuously-edited entities never settle → never versioned (unbounded, untested)
finding: 'The sweep filter updated_at < now()-$idle, with updated_at bumped on every write (entity.go:221/263/393), means an entity edited more frequently than the idle window NEVER satisfies the filter and is never snapshotted until editing stops. A hot entity edited steadily across a multi-hour session produces ZERO versions for the whole session; if it crashes or is deleted mid-session it has no intermediate history — the audit trail (S1/S3) is empty precisely for the most-active records. This is unbounded starvation, distinct from the accepted single-version debounce, and AC-7 only tests ''burst that DOES settle.'' Fix: add a max-staleness ceiling — also select entities whose latest version''s created_at < now()-$maxAge regardless of updated_at, forcing a snapshot of the current state of a long-running edit. Add an AC/test for the never-settles case. Document the debounce+ceiling semantics.'
severity: significant
resolution: 'Fixed: the sweep filter adds a max-staleness ceiling — an entity is selected when (updated_at < now()-$idle) OR (its latest version''s created_at < now()-$maxStaleness), regardless of ongoing edits. So a continuously-edited entity still gets a version snapshot of its current state at least every $maxStaleness. AC-8 tests a never-settling edit loop. Debounce+ceiling semantics documented.'
status: addressed
---
