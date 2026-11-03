---
id: RR-K1JELW
type: review-response
title: sqlite short-needle LIKE folds case for ASCII only
finding: sqlitestore search.go uses LIKE for needles under 3 runes. LIKE folds ASCII only while MatchTextFields folds Unicode, so Go can report a visible field the SQL did not match and admit a hit whose real match was a hidden field.
severity: minor
reason: Predates this bug and is specific to the sqlite short-needle path. It is a matcher-agreement defect, not a per-hit cost. Tracked as BUG-YG126C so the fix (lowercase the body or route short needles through FTS) gets its own differential test.
status: deferred
---
