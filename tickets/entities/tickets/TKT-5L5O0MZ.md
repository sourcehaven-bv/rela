---
id: TKT-5L5O0MZ
type: ticket
title: 'tenant: assemble every tenant over one SharedBase so config is parsed once'
kind: enhancement
priority: high
effort: s
started: "2026-11-07"
completed: "2026-11-07"
status: done
---

## Description

`tenant.AppBuildOpener` opens each tenant through `appbuild.New`
(`internal/tenant/registry.go:59`). `New` calls `prepare`, which parses the
metamodel, `acl.yaml` and the rest of the operator config again for every tenant
store it opens. The `Registry` doc says the config is parsed once for all
tenants, and acceptance criterion 3 of TKT-TNT9RS says the same. Neither is true
for this opener.

## Approach

- Build one `appbuild.SharedBase` at startup.
- Give the registry an opener that opens only the tenant's backend (pool,
store, search) for the tenant's DSN and calls `SharedBase.Assemble`.
- Keep the build-tag split: the backend open stays behind the build recipe.

## Acceptance criteria

1. Opening N tenants parses the config once. Pinned by a test that counts
parses or compares the shared `meta` and ACL policy pointers across leases.
2. `TestSharedBase_AssemblyDoesNotMutateSharedValues` and the postgres
isolation suite still pass.
3. The `Registry` doc matches the behaviour.

Found during the Pratique `org_id` multi-tenancy gap analysis (RES-D54281).
