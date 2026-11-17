---
id: RR-9YRHJL7
type: review-response
title: 'Test gaps: conditional relation and group routes with inheritance'
finding: No test for a conditional role relation plus inheritance (EdgeProps survive the strip), or for group routes with and without an ancestor.
severity: minor
reason: directOnly keeps Endpoints and EdgeProps untouched, and existing tests cover those fields on the run gate. A conditional fixture needs a compiled metamodel with relation properties; out of scope for this fix.
status: deferred
---
