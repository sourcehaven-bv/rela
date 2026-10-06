---
id: RR-B7AKJ8
type: review-response
title: Child view is fetched only to be discarded
finding: On a child URL the SPA fetches the full child view (sections and all), then redirects. The section work is wasted. Acceptable, but a view response could skip sections when it carries an owner.
severity: nit
resolution: 'Plan: the views handler skips building sections when it returns an owner; the SPA redirects before rendering sections.'
status: addressed
---
