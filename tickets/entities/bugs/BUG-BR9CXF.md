---
id: BUG-BR9CXF
type: bug
title: Tenant DSN derivation drops sslmode and other base_dsn settings, downgrading TLS to prefer
description: tenant.dsnForSchema re-serializes base_dsn keeping only host, port, user, dbname, password and search_path. sslmode=verify-full becomes the libpq default prefer (unverified TLS with plaintext fallback); pool_max_conns and other settings are lost.
priority: critical
status: backlog
---

## Summary

`tenant.dsnForSchema` derives each shared-tier tenant's DSN from `base_dsn` by
re-serializing only host, port, user, dbname, password and `search_path`, plus
`sslmode=disable` when TLS was off. Every other setting is dropped. When TLS was
on, no `sslmode` is written, so the derived DSN gets libpq's default, `prefer`.

Probe (2026-10-01, found while fixing BUG-JQO2PH):

```
base:    postgres://u:p@db.example:5432/rela?sslmode=verify-full&pool_max_conns=2&application_name=x
derived: host=db.example port=5432 user=u dbname=rela search_path=t1,public password=p
base:    InsecureSkipVerify=false, no plaintext fallback
derived: InsecureSkipVerify=true,  plaintext fallback
```

## Impact

- **TLS downgrade.** An operator who sets `sslmode=verify-full` (or
`verify-ca`/`require`) on `base_dsn` gets unverified TLS with a plaintext
fallback on every tenant connection. Credentials and tenant data are exposed to
an attacker on the network path.
- `sslrootcert`, `sslcert`/`sslkey`, `connect_timeout`, `application_name` and
`target_session_attrs` are lost too.
- `pool_max_conns` is lost, so the operator cannot size per-tenant pools,
which `tenant/registry.go` names as the binding resource.

The same re-serialization exists in `docscapture.dsnWithSearchPath` (dev docs
build), `backendtest.dsnWithSearchPath` and the pgstore listener test helper.

## Likely fix

Keep the operator's DSN and change only `search_path`, for example by setting
`cfg.RuntimeParams["search_path"]` on the parsed config and passing the config,
not a string. If a string is unavoidable, serialize from the parsed config
without dropping TLS, pool and connection keys, and add a round-trip test that
compares the parsed TLS config and pool settings before and after.
