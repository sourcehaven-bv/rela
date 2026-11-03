---
id: RR-3E9XVD
type: review-response
title: Service.Read duplicates Service.Messages page logic; HTTP keeps the old path
finding: Section 1.1 adds Read beside Messages and Head, but wirePage only borrows the cursors. Have the HTTP messages handler call Read with Signals and Schemas false, or have Messages delegate, so the clamp, the empty-head case and the cursor rule live once.
severity: minor
resolution: The HTTP messages handler calls Read with Signals and Schemas false, and Messages is removed or delegates.
status: addressed
---
