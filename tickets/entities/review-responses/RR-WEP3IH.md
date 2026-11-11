---
id: RR-WEP3IH
type: review-response
title: Cached 308 can loop after a rename back
finding: redirectRetired answered 308 Permanent Redirect with no Cache-Control. Browsers cache a 308 indefinitely. When an entity is renamed back onto its old id (which ends the retirement), or the reader loses access to the successor, the cached redirect still sends the old URL to the successor, and a rename back can produce a loop between the two ids.
severity: critical
resolution: 'The redirect is now 307 Temporary Redirect with Cache-Control: no-store, since it depends on the principal and can change after another rename. Tests assert both the status and the header on the views and entity GET routes; the guide documents why it is temporary.'
status: addressed
---
