---
id: RR-ICIU9
type: review-response
title: DocumentView drops router.back() fallback — bookmarked arrivals have no Back
finding: Pre-TKT-JIEKC DocumentView rendered a Back button that fell back to router.back() when no ?from= was present. Post-TKT-JIEKC (lines 54-57, 118) the button only renders when backTarget is non-null. A user bookmarking a document URL and arriving cold has no in-app Back affordance — must use browser Back. Ticket scope was 'honor return_to', not 'remove existing Back'. Either call this out explicitly in the ticket or restore a browser-back fallback. Low priority because browser Back is a universal path.
severity: nit
reason: 'Intentional behavior change. The pre-TKT-JIEKC Back button fell back to router.back() — which for a deep-linked arrival means ''go wherever the browser history happens to point,'' frequently nowhere useful (blank history, closing the tab). The new design: no Back button when no context exists. The browser''s own Back button handles the ''wherever you came from'' case; an in-app Back that pretends to know a source it doesn''t is a worse affordance. Documented in docs/data-entry.md under ''Back navigation'' (rule 3: neither return_to nor from present → no Back button renders).'
status: wont-fix
---
