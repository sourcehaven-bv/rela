---
id: BUG-07DNNY
type: bug
title: sqlite history attributes create/update versions to version-sweep instead of the editor
description: On the sqlite build create and update versions are attributed to the version-sweep system principal, so history loses who made the change (MCP, CLI and server alike). Postgres attributes them correctly.
priority: medium
effort: s
why1: sqlitestore's sweep.captureOne stamps every swept create/update version with the version-sweep system principal, because the live entities and relations rows it reads have no last_edited_by_user/_tool columns to copy the editor from.
why2: When sqlite versioning landed (TKT-4NU9ZD), the store's create/update writes did not persist store.AttributionFrom(ctx), although the entitymanager already puts it on ctx for every write. pgstore gained those columns in TKT-ZIRMGM, before sqlite versioning existed.
why3: The gap was recorded as an intended fallback in a sweep.go comment rather than as a parity defect, so it looked like a design decision rather than a missing feature.
why4: Swept-version attribution is pinned only by pgstore-internal tests (pgstore/attribution_test.go). storetest.RunVersionTests drives capture through the synchronous writer and explicitly excludes the sweep, so a second backend could pass conformance with no attribution at all.
why5: The version conformance suite treats the sweep as mechanism and not contract, but who is recorded as the author of a swept version is observable contract. Contract that only one backend's tests check is not enforced for the next backend.
prevention: The attribution contract moved into the shared store conformance suite (AM-sweep-attribution-conformance). A backend that declares versioning must supply a sweep driver, and the suite then fails if swept versions are not credited to the editor. A future backend can no longer ship this gap silently.
status: done
---

## Description

On the sqlite build, `rela history <id>` attributes every create and update
version to `unknown (version-sweep)`. The real editor is lost for every writer:
MCP, CLI and rela-server. On postgres the same MCP writes show `jeroen (mcp)`.
Rename and delete versions are attributed correctly, because they are captured
synchronously with the principal.

## Reproduction

1. `go build -tags sqlite -o rela-sqlite ./cmd/rela`
2. `rela-sqlite init` in an empty directory.
3. Run `rela-sqlite mcp` with `RELA_VERSION_SWEEP_INTERVAL=200ms RELA_VERSION_SWEEP_IDLE=200ms` and call `create_entity`, then `update_entity`.
4. `rela-sqlite history <id>` prints `create  unknown (version-sweep)` and `update  unknown (version-sweep)`.

The postgres build with the same steps prints `jeroen (mcp)` for both.
