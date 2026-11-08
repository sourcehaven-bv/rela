---
id: RR-R7LLLSM
type: review-response
title: 'Security: a completed relation keeps the listing endpoint verdict'
finding: A relation whose endpoint became hidden between ListRange and ListRelations still gets a hash of its meta.
severity: nit
reason: Documented. The listing already revealed the address and relation meta in scope is never hidden; the path runs only for unfilled relation elements.
status: wont-fix
---
