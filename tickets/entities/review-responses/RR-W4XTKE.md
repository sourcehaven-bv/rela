---
id: RR-W4XTKE
type: review-response
title: HighestID relied on LIKE's case-insensitivity by accident; DeleteEntity was four unsynchronized statements
finding: 'Two separate issues. (1) HighestID: SQLite''s LIKE is case-insensitive for ASCII by default, so HighestID(''FEAT'') matched ''feat-9999''. pgstore is byte-exact and re-checks strings.HasPrefix in Go; that guard was dropped, so the backends disagreed on ID generation. The slice id[len(pfx):] also assumed the matched prefix was the same length as the requested one — true today only by luck. (2) DeleteEntity: issued count → collect → DELETE relations → DELETE attachments → DELETE entities as separate auto-committing statements, so a failure between them left relations gone with the entity present, or orphaned attachments. Its check-then-act on relCount also raced a concurrent CreateRelation, since DeleteEntity took no writeMu.'
severity: significant
resolution: (1) Added an explicit EqualFold prefix guard with a comment recording that folding is DELIBERATE — IDs are case-insensitive identities per entities_id_lower_key — rather than LIKE's default doing it silently. (2) Wrapped DeleteEntity in s.Tx exactly as RenameEntity does, which also closes the check-then-act race; nesting is safe so callers already inside a Tx are unaffected.
status: addressed
---
