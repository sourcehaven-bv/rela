---
id: TKT-NJ91LX
type: ticket
title: Scheduled Lua jobs get the field redactor (closes RR-7408F5)
kind: enhancement
priority: medium
effort: s
tags:
    - security
status: backlog
---

## Description

`appbuild.ScheduledLuaWriteDeps` wires a **nil field redactor**
(`appbuild.go:415-426`), so scheduled Lua jobs get row-level gating only. A job
reading a `person` receives every property — including ones a human with the
same identity would see redacted in the UI.

That is not a considered trade-off, it is an accident of wiring. There is no
principled reason a scheduled job should see MORE than the same identity sees
interactively: `run_as` is an identity (DEC-O59WM4), and an identity's field
policy must not depend on which entry point happens to be reading.

Split out of TKT-XWZIOB: the fix is independent of `for_each` and worth landing
on its own, since it closes a live read-path gap for every existing deployment.

## It is wiring, not new machinery

`luaWriteDepsFor` already takes a `visibility.FieldRedactor` (`appbuild.go:403`)
and only ever gets `nil`. The redactor is
`visibility.NewPolicyRedactor(*affordances.PolicyResolver)` (`adapters.go:96`),
and `affordances.New(meta, lookup, declarative)` (`resolver.go:125`) needs
exactly three things `Services` already holds: the metamodel, a relation lookup
(the store), and `aclDeclarative` (`appbuild.go:115`). The dataentry equivalent
is `appRedactor` (`app.go:386`).

## Behaviour change

A scheduled Lua job that reads a `visible:`-restricted property **will stop
seeing it**. That is the point, but it needs calling out in the changelog rather
than landing silently — a script could be relying on the leak and would start
getting empty values.

With no ACL policy configured, behaviour must stay byte-identical:
`affordances.New` returns a resolver with a nil policy, which redacts nothing,
so the NopACL path is unaffected.

## Acceptance criteria

1. A scheduled Lua task reading a `visible:`-restricted property gets the
redacted value, matching what the same identity sees through the data-entry API.
2. With no ACL policy configured, scheduled reads are byte-identical to today.
3. Row-level gating is unchanged.
4. The changelog names the behaviour change.

## Risks

- **Silent behaviour change** — a script relying on the leak starts seeing empty
values. Intended; criterion 4 makes it visible.
- **Write-prep reads must stay raw** — entitymanager diffing needs unredacted
access, or a read-modify-write would clobber hidden fields. This ticket changes
the *script read* seam only.
