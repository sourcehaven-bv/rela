---
id: BUG-UO72G2
type: bug
title: RELA_AFFORDANCE_PROFILE=none/demo disables client field ceilings on REST
description: The none or demo affordance profile replaces the policy resolver on REST, so client field ceilings are not applied there while MCP and CLI still redact.
priority: medium
effort: s
why1: ResolverFromProfile returned the Nop or demo resolver for none/demo before it looked at the policy so REST ignored client field ceilings and role field grants.
why2: DR-M3 (PLAN-K0SF) made the profile a hard override when field rules lived only in role blocks and the REST resolver was the only place they applied.
why3: Client field ceilings (TKT-IAC8TX) and the MCP/CLI field policy (appbuild buildFieldPolicy) were added later and read the policy directly; nobody revisited the override.
why4: DR-M3 was recorded only as a planning-checklist note and not as a decision entity so later work on field rules had nothing to link against.
why5: Env-var overrides that bypass a policy are not checked against the policy they bypass; a developer knob can silently weaken a security rule on one transport.
prevention: ResolverFromProfile now refuses none/demo when the policy declares field rules and startup fails (pinned by TestResolverFromProfile_Selection and TestNewServer_AffordanceProfileWithFieldRules). A development override must fail loudly when it would weaken a declared policy rather than replace it silently.
started: "2026-10-31"
completed: "2026-10-31"
status: done
kind: bug
---

## Description

`ResolverFromProfile` (internal/dataentry/affordances_stub.go) applies the
`RELA_AFFORDANCE_PROFILE` override before it looks at the policy (DR-M3). With
`none` or `demo`, REST uses a resolver that ignores `client_baselines` field
ceilings, while MCP and CLI reads (appbuild `buildFieldPolicy`) ignore the
profile and still redact. Found in the code review of BUG-KJ5D18 (GitHub #1884).

## Decision

Option 1 was chosen: refuse `none`/`demo` at startup when the policy declares
any field rule (`acl.Policy.HasAffordanceGrants()`: role `fields:`, `visible:`,
`options:`, `relations:`, or a client field ceiling).

The options considered were:

1. Refuse `none`/`demo` at startup when the policy declares a client field
ceiling (or any field grant).
2. Keep the override but log an error naming the ceilings that are not applied.
3. Keep DR-M3 as is and document the bypass.

## Reproduction

1. Write an `acl.yaml` with a `client_baselines` entry that redacts
`ticket.status` for `applies_to: [app]`.
2. Start `rela-server` with `RELA_AFFORDANCE_PROFILE=none`.
3. GET a ticket as an app-type principal over REST: `status` is returned.
`rela` MCP/CLI reads of the same entity redact it.

## Fix

`ResolverFromProfile` reads `HasAffordanceGrants()` first. For `none` or `demo`
with grants it returns an error wrapping
`dataentry.ErrProfileOverridesFieldRules`. The message names
`RELA_AFFORDANCE_PROFILE`, the profile value, and says that acl.yaml declares
field rules. `rela-server` already fails startup on a resolver error (the DR-M4
compile-error path: `buildApp` → `build` → `newServer`), and so does a Configure
save, which keeps the old generation serving. `rela-desktop` surfaces the same
error through `failLoad`, but it wires `acl.NopACL`, so it has no declarative
policy and the profiles keep working there.

Without a declarative, or with a policy without field rules, both profiles
behave as before. An unknown profile still warns and falls through.

## DR-M3 addendum

DR-M3 is not an entity; it is a note in PLAN-K0SF ("Treat `none` as a hard
override ... even when policy has affordance blocks"). It is amended by this
bug: `none` and `demo` stay hard overrides only when the policy declares no
field rules. With field rules present they are a startup error, because the
override applied to REST alone while MCP and CLI kept the rules.
