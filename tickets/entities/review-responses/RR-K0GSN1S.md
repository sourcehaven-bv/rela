---
id: RR-K0GSN1S
type: review-response
title: Check order could reveal hidden runs
finding: A 409, 422 or 403 answered before the header gate reveals a run exists.
severity: significant
resolution: 'Order: identity, header gate (uniform 404), run store, then generation, writes and roles. SubjectRuns gates each run''s header in one batch instead of trusting the subject''s readability, so a run on another face stays hidden. Table case added.'
status: addressed
---
