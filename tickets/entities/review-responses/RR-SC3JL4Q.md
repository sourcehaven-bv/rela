---
id: RR-SC3JL4Q
type: review-response
title: A rejoin that keeps failing is retried every 30s without growth
finding: The shape-holding case of joinDue always returns at+minBackoff, and branchesBackedOff only backs off branches, so a persistent store error on the rejoin is hit every 30s.
severity: minor
reason: Filed in TKT-7VAHS94 with the other join-poll costs. A store that fails every write also fails every other run, so this adds load in proportion to an outage that already exists.
status: deferred
---
