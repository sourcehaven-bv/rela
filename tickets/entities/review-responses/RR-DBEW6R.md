---
id: RR-DBEW6R
type: review-response
title: Swallowed auto-link failure is invisible on the add-another path
finding: The link_as='from' auto-link createRelation (:1150) is caught and swallowed with only a console.warn (:1152-1155). On the navigate path the user lands on the entity and can see the missing link; on the add-another path they never see it, so repeated entry can silently produce N unlinked entities. Surface it as a toast in 'again' mode.
severity: significant
status: addressed
resolution: >-
  New plan step 4a: surface the swallowed auto-link failure as a toast on the 'again' path.
---
