---
id: RR-2HX7I3
type: review-response
title: A step can land on another tail of the same source
finding: When one source links from two readable faces, the list shows it once, but the order holds two places. A step that only moves past the other tail looks like nothing happened.
severity: nit
reason: The client sends before/after by name for every step inside a page; only a step at a page edge is sent as a step. A source linking from two readable faces into one list is rare.
status: wont-fix
---
