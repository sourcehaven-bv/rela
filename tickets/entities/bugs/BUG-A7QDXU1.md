---
id: BUG-A7QDXU1
type: bug
title: Flow run hidden from role-relation reader when inherit_roles_through is set
description: A reader who holds a role on a flow run's subject through a direct role_relations edge cannot read the run, see its task or complete its step when acl.yaml declares inherit_roles_through.
priority: high
effort: s
why1: The flow run read gate dropped the subject type's branch, so the run matched no branch for a role-relation reader.
why2: gateHop refuses a read query whose HasInbound carries an inheritance expansion, and every conferred read query carries one once acl.yaml declares inherit_roles_through.
why3: conferredEdge adds the expansion to every role-relation predicate, and flowRunReadQuery reused the traversal fold, which must refuse rather than narrow, instead of narrowing to the direct edge.
why4: The flow run tests covered role relations and inheritance separately but never a policy with both.
why5: The run gate reuses the traversal fold, whose contract is to refuse what it cannot express, for an allowlist where narrowing is the safe answer; nothing tested a policy that combines the features the fold refuses with the ones it allows.
prevention: Regression tests in internal/acl and internal/appbuild run a role-relation reader with inherit_roles_through set, and check who-can routes against the gate.
started: "2026-11-17"
completed: "2026-11-17"
status: done
---

## Description

A person who reads a flow run's subject only through a `role_relations` edge
cannot see the run, its task, or complete its step, when `acl.yaml` declares
`inherit_roles_through`. `POST /_flows/runs` returns 201, but `GET` on the run
returns 404, the run and task lists are empty, and `_complete` returns 404.
`rela acl who-can read` on the run lists only global-role readers.

Seen in harbourmaster: a customer linked to their account by `customer-of`
cannot do their own signup step.

## Cause

`conferredEdge` puts the `inherit_roles_through` expansion on every conferred
read predicate. `flowRunReadQuery` folds each subject type's read query into a
`_carries` endpoint through `gateHop`, which refuses an inheritance expansion.
So the whole subject-type branch was dropped, including direct edges.

## Fix

`flowRunReadQuery` strips the inheritance expansion from the subject's read
query before folding it (`directOnly`). A direct role-relation edge on the
subject now admits the run; a subject read only through an ancestor still hides
its runs, as documented. `flowRunReadRoutes` skips ancestor routes, so who-can
matches the gate.
