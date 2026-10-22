---
id: BUG-UO72G2
type: bug
title: RELA_AFFORDANCE_PROFILE=none/demo disables client field ceilings on REST
description: The none or demo affordance profile replaces the policy resolver on REST, so client field ceilings are not applied there while MCP and CLI still redact.
priority: medium
status: backlog
kind: bug
---

## Description

`ResolverFromProfile` (internal/dataentry/affordances_stub.go) applies the
`RELA_AFFORDANCE_PROFILE` override before it looks at the policy (DR-M3). With
`none` or `demo`, REST uses a resolver that ignores `client_baselines` field
ceilings, while MCP and CLI reads (appbuild `buildFieldPolicy`) ignore the
profile and still redact. Found in the code review of BUG-KJ5D18 (GitHub #1884).

## Decision needed

DR-M3 makes the profile a deliberate hard override. Options:

1. Refuse `none`/`demo` at startup when the policy declares a client field
ceiling (or any field grant).
2. Keep the override but log an error naming the ceilings that are not applied.
3. Keep DR-M3 as is and document the bypass.
