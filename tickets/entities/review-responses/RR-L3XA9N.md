---
id: RR-L3XA9N
type: review-response
title: Sidebar ACL check rebuilt the principal scope once per entity type
finding: inlineCreateForms called computeCollectionActions per entity type, which routes through Declarative.AuthorizeWrite → ForPrincipal (declarative.go:207) — rebuilding the memoized Request scope every iteration and discarding it. Role resolution walks `member-of` through the store, so an N-type metamodel paid N graph traversals on every sidebar fetch (i.e. every app load), where the list handler pays one.
severity: significant
resolution: inlineCreateForms now resolves the scope ONCE via acl.FromContext(ctx) (the middleware already attaches it) and reuses it across the loop, falling back to the unscoped path when absent (tests / non-HTTP callers) — same answer, slower. The form lookup still runs first so a type with no form costs no authorization at all.
status: addressed
---
