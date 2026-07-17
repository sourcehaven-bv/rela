---
id: RR-SH28E
type: review-response
title: Sweep silently re-captures purged content within one interval if the live row still holds it
finding: 'THE killer flaw (cranky C2). Purge deletes version rows but does NOT touch the live entity/relation. The reconciliation sweep (sweep.go, default 5min) probes ''does current live content differ from the newest version?'' — after a --all purge there are ZERO version rows, so the sweep sees divergence and RE-INSERTS a fresh create/update version capturing the current live content, PII included. So purge-without-live-redaction is silently undone within one sweep interval, with a NEW version row (version-sweep principal) that looks normal. An operator who ran purge, saw ''purged 5 rows'', and told a DPO ''erasure complete'' has told a falsehood. Fix (cranky L1, the good design): purge takes the SAME sweep advisory lock (mutual exclusion with the tick) AND, if a live row exists, either (a) REFUSES with ''live row still holds this content; redact the live value or delete the entity first'', or (b) writes a no-content `purge` tombstone version row (op=purge, principal, vseq range, NO content) that the sweep''s newest-version probe recognizes as ''deliberately purged, do not re-capture''. Option (b) turns purge from ''a deletion the sweep undoes'' into ''a deletion the sweep respects'' and doubles as an in-band forensic marker. The ticket''s ''Does NOT touch the live entity/relation'' safety claim is actually THE BUG.'
severity: critical
resolution: 'Design revised: purge takes the sweepAdvisoryLockKey (mutual exclusion) and REFUSES by default when a live row exists; --force-live writes a no-content `purge` tombstone the sweep respects. See revised design #1.'
status: addressed
---
