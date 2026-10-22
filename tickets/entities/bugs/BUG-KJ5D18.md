---
id: BUG-KJ5D18
type: bug
title: Client field ceilings are ignored when no role declares field grants
description: 'ResolverFromProfile picks the Nop resolver when no role declares visible:, fields: or options: grants, so a policy whose only field restriction is a client_baselines ceiling redacts nothing. Raised by the CISO on #1882 from RR-IAO5NC.'
priority: high
why1: Policy.HasAffordanceGrants returned false for a policy with only client_baselines field ceilings, so appbuild and dataentry chose NopRedactor and NopFieldVerdictResolver.
why2: The ceiling is applied only inside affordances.PolicyResolver (applyClientCeiling and SearchFieldScope), and the selection predicate only looked at role blocks.
why3: Client attenuation (TKT-IAC8TX) added a new source of field restrictions without updating the resolver-selection predicate that predates it.
why4: The predicate's name and doc framed it as role-only, so nothing tied a new field-restricting config key to it.
why5: 'Tests for ceilings always used policies that also had role visible: blocks, so the Nop branch was never exercised with a ceiling.'
prevention: HasAffordanceGrants now counts baseline field ceilings and its doc names every field-restriction source; tests pin a ceiling-only policy on appbuild reads and search and on the HTTP API (MEAS-CLIENT-CEILING-ONLY-POLICY); ceiling field keys that could never match (* or type@face) now fail policy load instead of loading inert.
started: "2026-10-22"
completed: "2026-10-22"
status: done
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
