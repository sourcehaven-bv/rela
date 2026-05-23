---
id: RR-4Q0T
type: review-response
title: Predicate for entry-content section duplicated between find and map
finding: entryContentSection uses find (first match), nextSections uses map (rewrites every match). Today server guarantees one match; user-authored views with multiple entry-source content sections would silently break. Extract predicate to a named helper used by both.
severity: minor
resolution: Extracted `isEntryContentSection(s)` helper used by both `entryContentSection` (find) and `handleCheckboxToggle` (map). Single source of truth for the predicate; future schema changes only require updating one place.
status: addressed
---
