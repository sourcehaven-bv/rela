---
id: TKT-ZJXO19
type: ticket
title: Regression test for empty FromType against a type-scoped policy
kind: test
priority: medium
effort: s
status: done
---

## Description

`CreateRelation` computes a best-effort — possibly empty — `FromType` **before**
the ACL check, so authorization does not depend on whether the source entity
exists yet (BUG-K6FEVB). The existing regression tests exercise that only
against `acl.ReadOnlyACL` (denies everything) and `acl.NopACL` (allows
everything). Neither is type-scoped, so neither can show what an empty
`FromType` does against a realistic `acl.yaml`-shaped policy with specific
per-type grants.

GitHub issue #1129 (IB-review rela#1115). Basis: CONTROL-5-15; the rela ACL has
been in production since 2026-07-07.

## The claim under test

Code inspection of `grantsVerb` says an empty `FromType` can only match a
wildcard grant (`t == "*"`, already permissive without the change) or a literal
empty-string grant (not a realistic configuration) — so it can only ever be
*more* restrictive than the real type, never more permissive.

That reasoning looks right. But it is currently **prose only**: nothing fails if
a future change to `grantsVerb`, or to how `FromType` is derived, makes an empty
type match something it should not.

## Scope

A test that calls the relation write paths with a **non-existent source entity**
against a policy with specific (non-wildcard) type grants, and asserts the
outcome is never *more* permissive than the same call with the correct
`FromType`.
