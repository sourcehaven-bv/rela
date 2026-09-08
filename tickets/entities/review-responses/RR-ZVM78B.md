---
id: RR-ZVM78B
type: review-response
title: The *acl.Declarative assertion fails OPEN on an unrecognized type, unlike dataentry's switches
finding: 'The predicate is correct today (both reviewers independently enumerated all four ACL implementations and confirmed nothing slips through), but it is a third divergent spelling of ''is a policy active''. internal/dataentry asks the same question as a switch on NopACL/ReadOnlyACL whose default arm fails CLOSED; the is-Declarative assertion falls through to ''no policy'', i.e. OPEN. A future decorating implementation — an audited or multi-tenant wrapper — would make dataentry hide a UI element while entitymanager waves an ungated copy read through: the forgotten-wiring-becomes-a-bypass shape (RR-X9NVHI) this change closes, reintroduced one wrapper later.'
severity: minor
resolution: 'Took the reviewer''s third option, the one matching how this repo already handles clamp-point drift (ceilingguard_test.go): added TestACLImplementations_AreAClosedSet in internal/acl. It scans the package for AuthorizeWrite implementations against an allowlist, so a new implementation fails the build with a message naming every is-policy-active site to update; it also flags a stale entry if one is removed. Request is listed explicitly as satisfying the interface while never being wireable as a deployment ACL. Verified non-vacuous by adding a decorating auditedACL, which the guard caught. requireCopyGates now carries a comment stating that the assertion is safe only because the set is closed, and pointing at the guard.'
status: addressed
---
