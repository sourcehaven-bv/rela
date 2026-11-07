---
id: TKT-SNVR8UV
type: ticket
title: 'tenant: give each tenant its own audit sink'
kind: enhancement
priority: medium
effort: s
status: backlog
---

## Description

`SharedBase.OpenAndAssemble` assembles every tenant over one base, and the base
holds one `Config.Audit`. Every tenant therefore writes its audit records to the
same sink. With `appbuild.Discover` that is `<CacheDir>/audit`, so the audit
logs of all tenants are interleaved in one file set. The old `AppBuildOpener`
had the same behaviour.

## Approach

Make the audit sink a per-tenant input of `OpenAndAssemble`, or have the host
pass a sink that routes by tenant. On postgres, an audit sink in the tenant
schema is the natural fit.

## Acceptance criteria

1. A write through tenant A is recorded only in tenant A audit log.
2. A test opens two tenants and reads each audit log.

Found in the code review of TKT-5L5O0MZ.
