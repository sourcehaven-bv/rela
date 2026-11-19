---
id: RR-IEZSQ7S
type: review-response
title: Load more could return an empty page
finding: A full page returned a cursor even when every later record was left out.
severity: minor
resolution: EntityChanges reads on until a later shown record exists before returning a cursor; TestEntityChanges_NoCursorBeforeOnlyHiddenRecords.
status: addressed
---
