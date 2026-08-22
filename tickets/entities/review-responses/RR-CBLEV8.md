---
id: RR-CBLEV8
type: review-response
title: 'Checkbox visual is now drawn in two files; the sanctioned fix is a shared class, not a second copy'
finding: 'CheckboxWidget and RelationCards now each draw the same custom checkbox, and the copies have already diverged (`4px` vs `var(--radius-sm)`, `white` vs `#fff`, prefixed vs unprefixed appearance) while sharing one bug (the hardcoded indigo focus ring, RR-CBS2QW). frontend/CLAUDE.md documents this exact trajectory for `properties-list.css`: those classes "used to be declared three times with three different min-width values, scoped so they could never actually share". A shared `.rela-checkbox` in src/styles/ would delete ~35 duplicated lines and make the ring a one-line fix in one place.'
severity: minor
resolution: 'Deferred to a follow-up ticket rather than done here.'
reason: 'The extraction means editing RelationCards — a working component with its own inline-edit behaviour — to serve an `effort: xs` styling ticket, and it changes a second surface that this ticket''s acceptance criteria never exercise. Doing it inside this diff would make the change unreviewable as a styling fix. Recorded explicitly on the ticket (Follow-ups) rather than left implicit, which is what the reviewer asked for if deferred.'
status: deferred
---

Note the divergences are not symmetric: the widget is now the *better* copy
(theme-following ring, forced-colors fallback, correct disabled specificity).
So the follow-up is "lift the widget's version into a shared class and adopt it
in RelationCards", not a merge of equals.
