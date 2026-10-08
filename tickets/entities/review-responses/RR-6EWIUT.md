---
id: RR-6EWIUT
type: review-response
title: Absent vs empty segments ambiguous
finding: omitempty drops an empty segment list (all-code range) and the client falls back to marking the whole range.
severity: minor
resolution: 'Plan revised: segments sent without omitempty for located text anchors; [] means mark nothing; absent means older server.'
status: addressed
---
