---
id: RR-GR3C
type: review-response
title: Pull readGate consumer-side interface forward into PR 1
finding: 'PR 2''s plan flags readGate as ''optional but recommended.'' Not optional in any real sense: PR 1 wires ACL into 4-5 probe sites (GET, PATCH, DELETE, Action, includes); PR 2 adds 3+ more (list, sidebar, _position). If PR 2 introduces readGate and only uses it for list paths, PR 1''s sites become inconsistent stragglers — RR-CB8Y''s whole concern. Extracting readGate in PR 1 (~30 LOC: one consumer-side interface in internal/dataentry, one production impl wrapping *acl.Request, one NopGate) makes PR 2 mechanical. Doing it in PR 2 forces a refactor across PR 1 sites in PR 2''s diff, inflating PR 2''s review surface — exactly what the split was supposed to avoid. Pin: introduce readGate in PR 1 and use it everywhere. Update RR-CB8Y status to addressed-in-PR1.'
severity: significant
status: open
---
