---
id: RR-WUNVER
type: review-response
title: CheckReload refuses changes for unrecorded types
finding: It refuses an anchor or cardinality change for a type that never had a channel, contrary to the design (compare recorded types only).
severity: minor
reason: Not resolved by the service commits. The record check (checkRecords) compares recorded types only, but Reload still runs realtimecfg.CheckReload(r.last, next) first, which compares every type of the previous config, so a reload still refuses an anchor or cardinality change for a type with no channels; a restart accepts it. The check is stricter than the design but never strands data. Dropping CheckReload from Reload now that records cover the case, and aligning docs/realtime.md, is a follow-up.
status: deferred
---
