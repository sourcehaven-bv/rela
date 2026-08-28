---
id: AM-acl-builtin-permissions-audit-exempt
type: automated-measure
title: Every rela-shipped global permission constant is registered and A7-exempt
description: 'A test that enumerates acl.BuiltinPermissions() and asserts (a) it is non-empty, and (b) running the A7 dead-permission check over a minimal policy granting each constant emits no finding. Registration — not a literal list inside aclaudit — is what makes this cover future built-ins automatically: adding a new global permission constant without registering it fails the test rather than silently producing a false ''dead'' report in operator configs.'
kind: test
location: internal/aclaudit (table test over acl.BuiltinPermissions())
status: proposed
---

Guards the why5 of the built-in-permissions bug: the audit assumed `acl.Policy`
was the complete world of permission *consumers* as well as producers, so
anything consumed outside `acl.yaml` became "dead" by construction.

A literal two-constant exemption inside `aclaudit` would fix today's symptom and
re-break on the next constant added. Driving the exemption off an exported
`acl.BuiltinPermissions()` moves the failure to CI, at the moment the constant
is introduced, instead of into an operator's audit output months later.

The (b) assertion matters more than (a): it tests the *behaviour* an operator
sees, not merely that a list is populated.
