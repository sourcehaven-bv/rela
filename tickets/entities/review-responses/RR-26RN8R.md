---
id: RR-26RN8R
type: review-response
title: RelationHistoryQuery.Key doc contradicts the code
finding: The doc said the tail is ignored when RecordID is set; recordIDIsHeadOfKey filters on from_face.
severity: minor
resolution: Doc now says the RecordID is validated against the whole key, tail included.
status: addressed
---
