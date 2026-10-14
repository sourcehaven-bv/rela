---
id: BUG-KJ5D18
type: bug
title: Client field ceilings are ignored when no role declares field grants
description: 'ResolverFromProfile picks the Nop resolver when no role declares visible:, fields: or options: grants, so a policy whose only field restriction is a client_baselines ceiling redacts nothing. Raised by the CISO on #1882 from RR-IAO5NC.'
priority: high
status: backlog
---

## Description

`appbuild` (about `appbuild.go:1286`) chooses `NopRedactor` and the Nop
affordance resolver when the policy has no role-level field grants. A
`client_baselines` entry that hides fields from an app client is then never
applied: the field ceiling is fail-open.

Found as RR-IAO5NC on BUG-WJSYQT (deferred, unverified) and raised by the CISO
review of #1882 (CONTROL-8-08).

## To do

1. Reproduce with a policy whose only field restriction is a client ceiling.
2. Choose the declarative resolver whenever any client ceiling hides or redacts a field.
3. Add a golden or acl test pinning the ceiling under that policy.
