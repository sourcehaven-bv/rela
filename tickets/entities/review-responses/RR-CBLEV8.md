---
id: RR-CBLEV8
type: review-response
title: 'Checkbox visual is now drawn in two files; the sanctioned fix is a shared class, not a second copy'
finding: 'CheckboxWidget and RelationCards now each draw the same custom checkbox, and the copies have already diverged (`4px` vs `var(--radius-sm)`, `white` vs `#fff`, prefixed vs unprefixed appearance) while sharing one bug (the hardcoded indigo focus ring, RR-CBS2QW). frontend/CLAUDE.md documents this exact trajectory for `properties-list.css`: those classes "used to be declared three times with three different min-width values, scoped so they could never actually share". A shared `.rela-checkbox` in src/styles/ would delete ~35 duplicated lines and make the ring a one-line fix in one place.'
severity: minor
resolution: 'Superseded by [[RR-CBSHARE]] — done in this ticket after all, and better than proposed: RelationCards now renders the shared CheckboxWidget rather than adopting a shared stylesheet, so behaviour and accessibility are shared too, not just paint.'
reason: 'Originally deferred on scope grounds (editing a working component to serve an `effort: xs` styling ticket, on a surface the acceptance criteria never exercise). The user overrode that once the tick fix landed, and was right to: RR-CBTICK9 corrected the tick in the widget only, leaving the duplicate in RelationCards visibly wrong in the product. A fix that reaches one of two identical surfaces is worse than the scope cost of doing both.'
status: addressed
---

Note the divergences are not symmetric: the widget is now the *better* copy
(theme-following ring, forced-colors fallback, correct disabled specificity).
So the follow-up is "lift the widget's version into a shared class and adopt it
in RelationCards", not a merge of equals.
