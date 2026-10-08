---
id: RR-D2HEFA
type: review-response
title: Ambiguous address swallowed in ref lookup
finding: A ref on a non-first face is never found; the connector then 422s for good.
severity: minor
resolution: 'Lookup reads each candidate at the face holding the ref and returns other errors. Test: TestFindByExternalRef_Faces.'
status: addressed
---
