---
id: RR-EE47LA
type: review-response
title: 'State-machine when: is per machine; not per type'
finding: A machine can be shared by several entity types; validating and binding against one type is wrong for the others.
severity: minor
resolution: Validation runs per (type; property) pair using the machine; Bind groups ids by type; a narrow traversal interface is used rather than growing GraphLookup; test with a machine shared by two types.
status: addressed
---
