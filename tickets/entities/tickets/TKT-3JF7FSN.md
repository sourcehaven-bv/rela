---
id: TKT-3JF7FSN
type: ticket
title: Apply pending config migrations in memory on load and warn instead of refusing to boot
kind: enhancement
priority: high
effort: m
started: "2026-11-25"
completed: "2026-11-25"
status: done
---

## Description

A deployment of a new rela version refused to boot because the existing config
used syntax that a migration in `internal/migration` rewrites. The fix was
available (`rela migrate`), but the server stopped instead of using it.

Today the loaders refuse on any pending migration:

- the schema loader (`internal/metamodel/loader_service.go`) on every
migration that is not `Advisory`;
- the data-entry loader (`loadConfig` in `internal/dataentry/app.go`) on every
detection, ignoring `Advisory`.

`Advisory` only helps when the parser still accepts the old syntax. Most
migrations exist because the parser no longer does, so downgrading the error
alone would fail in the parser instead.

## Proposal

At boot and on hot reload, when a file has pending migrations:

1. Apply them to the in-memory `yaml.Node` tree. Never write to disk.
2. Parse the migrated tree with the normal, strict parser.
3. Log one warning per file naming the migrations and telling the operator to
run `rela migrate`.
4. Report pending migrations on the health/admin surface too.

The parser stays strict; it only ever sees migrated output. Every migration is
kept in the registry permanently already, so this adds no new legacy surface.

## Scope

- Schema and data-entry files. ACL and flows files if their loaders gain
detection.
- Pass the loaded schema to `WithMetamodel` when migrating data-entry
(`dataentry_cleanup`, `relation_direction` need it).
- Project-level migrations that span files (`automations_to_flows`) stay hard
errors for now.
- Make the data-entry loader treat `Advisory` the same way as the schema
loader.
- `rela validate` keeps failing on deprecated syntax so CI still catches it.
Consider a strict server flag for operators who want the old refusal.

## Acceptance

- A project with pre-migration schema and data-entry syntax boots, serves the
same behaviour as after `rela migrate`, and logs a warning per file.
- The files on disk are unchanged.
- Hot reload of a file with old syntax succeeds with the same warning.
- A cross-file migration still refuses boot with the current message.
- `rela validate` still fails on deprecated syntax.
