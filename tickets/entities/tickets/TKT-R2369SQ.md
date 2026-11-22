---
id: TKT-R2369SQ
type: ticket
title: 'seqtrace routes suite: trace the audit, changes and next-action routes'
kind: chore
priority: low
effort: xs
status: done
---

## Description

The weekly traced archplan run (issue #2369) reported five new security
findings. Three entry points are never called in the traces:
`handleV1Audit`, `handleV1Changes` and `handleV1NextActionPost`. The guard
`acl.HoldsPermissionOnAnyEntity` is never called, and no trace shows
`handleV1NextActionGet` refusing a caller.

Extend the routes suite in `tools/seqtrace/demo/run.sh`:

- GET `/api/v1/_audit` as the reader (403) and as the manager (200). The
  manager gets `audit:read` in the project copy.
- GET `/api/v1/_changes/task/{id}` as the reader (200), and for a risk the
  reader may not read (404).
- POST `/api/v1/_next_action` as the reader (204), and with an unknown
  source (400).
- GET `/api/v1/_next_action` with an unknown surface (400). The read is an
  ACL-scoped list and refuses no principal, so this is its one refusal.
- A `permission:` on the task board nav entry, so the sidebar asks
  `HoldsPermissionOnAnyEntity`.

Two fixes outside the suite make those traces count:

- seqtrace named a diagram only after a method handler (`\.handle`), so a
  function handler such as `handleV1Audit` was never the entry of its trace.
  The title now also matches `^handle[A-Z]`.
- With that, the SPA shell trace was the only one left under the
  middleware, which has no auth declaration. The shell closure becomes the
  named handler `spaShell.handleShell`, declared `auth=none`.

`handleV1NextActionPost` was declared `auth=authz`, but it writes only the
caller's own snoozes and mutes, so it is declared `auth=required`.

## Acceptance criteria

1. Each new request answers the status its `EXPECT` names.
2. A traced `archplan ci` reports none of the five findings of #2369.
