---
id: TKT-EFOQCX
type: ticket
title: Script readers bind the ACL scope per call instead of per operation
kind: refactor
priority: low
effort: s
status: backlog
description: ScriptReader.bind drops bind failures and lateGatedReader rebuilds the gated reader on every call.
---

## Description

Deferred from TKT-5LW875 review (RR-W2RINP). Two pre-existing behaviours in the
script read path:

- `visibility.ScriptReader.bind` falls back to the unbound ctx when
`binder.Bind` fails. The gate still denies on its own terms, so visibility never
widens, but the failure is silent and every later read in the operation re-walks
memberships.
- `dataentry.lateGatedReader` builds a new gated reader on every call
(`reader()` in `internal/dataentry/app.go`). Each scan gets its own ACL scope,
so one script operation does not share one membership walk. This conflicts with
"capture state once per operation" and with the rule that authorization reuses
the request's `acl.Request`.

Per-call construction exists so a policy reload or a test rebind of `a.acl` /
`a.fieldResolver` is honoured.

## Approach

- Resolve the reader once per operation (per script run or request) rather
than once per call, keeping reload semantics at operation granularity.
- Log a bind failure once, with the principal, instead of dropping it.

## Acceptance

- One script operation performs one ACL scope bind. Test: a counting budget
on a multi-read script.
- A bind failure is logged and still fails closed.
