---
id: RR-T45UZGL
type: review-response
title: Every run change refreshes pages with a flow_tasks source
finding: Pages with a flow_tasks source re-resolve on any run move by anyone.
severity: minor
resolution: 'Documented at refreshTypes: the client debounces, and a resolve reads one task list plus one subject batch per type. Narrowing to the reader''s runs needs events that name them.'
status: addressed
---
