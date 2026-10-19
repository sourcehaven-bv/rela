---
id: RR-RNYIVB
type: review-response
title: The `Gone` marker is dead state; use it to fix D10 instead
finding: In the §3 table, "absent or Gone + matches" posts `enter` either way, so `Gone`, SeenGrace and the Gone-GC only add complexity. Either delete the entry when the row is absent, or keep it and treat "Gone + matches + same fingerprint" as a silent re-match, so a soft-delete restore does not post `enter` again (a better D10).
severity: minor
resolution: Per D10 restore posts again. Gone and SeenGrace are removed; an absent row deletes its entry.
status: addressed
---
