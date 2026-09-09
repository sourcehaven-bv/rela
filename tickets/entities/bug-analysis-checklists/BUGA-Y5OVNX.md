---
id: BUGA-Y5OVNX
type: bug-analysis-checklist
title: 'Analysis: rela init generates a schema.yaml that fails to load, making new projects unusable'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally
- [x] Minimal reproduction steps documented
- [x] Environment/conditions noted

`mkdir /tmp/initest && cd /tmp/initest && rela init && rela migrate status`
printed:

```
/tmp/initest/schema.yaml uses deprecated syntax:
  - Add explicit id_type: sequential to entities without id_type, rename deprecated values
Run 'rela migrate' to update your project files.
```

`rela validate` passed on the same file, which initially suggested the schema
was merely stale rather than broken. That was wrong, and code review caught it:
`validate` and `migrate status` each run their own path, but
`metamodel.FSLoader.Load` turns any detection into a `*migration.Error`.
Re-tested against a binary built from the parent commit: `rela list requirement`
in a freshly-initialized directory failed with the same "uses deprecated syntax"
error. Every command was affected, so a new project could not be used at all.

Environment: default (fs) build, clean empty directory. Nothing
environment-specific.

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

The four entity definitions in `DefaultMetamodelYAML()`
(`internal/metamodel/loader.go`) carried `id_prefix` but no `id_type`.
`ShortIDDefaultMigration.Detect` returns true for any entity missing `id_type`,
so the generated file matched the migration on its first read, and
`FSLoader.Load` refused it.

The workaround the error suggested made things worse rather than better: running
`rela migrate` writes `id_type: sequential`, so the only path to a loadable
project silently pinned it to legacy sequential IDs — the opposite of the
`short` default the project was supposed to get.

## Fix Planning

- [x] Fix approach determined
- [x] Regression test planned
- [x] Related areas checked for similar issues

Fix: write `id_type: short` explicitly in the template. Deliberately `short`,
not the `sequential` the migration inserts — the migration preserves legacy
behavior for projects predating the default change, whereas a new project should
get the current default. `GetIDType()` already resolves an empty value to
`short`, so the change is behaviour-neutral for existing projects. Verified a
fresh init now mints `REQ-CGLP`.

Regression test asserts `projectsetup.DetectMigrationsWithFS` finds nothing on
the output of the real `Initialize`, covering every file type `rela migrate
--check` scans against the whole registered migration set rather than this one
migration.

Related areas checked — the first pass here was too narrow, and code review
found three more copies of the same defect:
- `scripts/generate-test-data.sh` embeds a stale fork of the template, missing
`id_type` on all four entities. Fixed.
- `prototypes/data-entry/schema.yaml`, `catalog/schema.yaml` and
`catalog-metamodel.yaml` each omit `id_type` on one prefixed entity, so those
prototypes did not load. Fixed, along with a `link: true` migration that was
also blocking the catalog prototype.
- `docs/metamodel.md` actively taught the defect by showing `id_type` commented
out as "the default"; `docs/cli-reference.md` stated the migration's rename
direction backwards. Both fixed, with their `docs-project/` sources.
- `DefaultMetamodel()` (the Go-struct twin) leaves `IDType` empty, which resolves
to `short`, so it already agrees with the fix. Verified with a throwaway test.
- `Initialize` writes only `schema.yaml`, so no acl.yaml/data-entry.yaml starter
template exists to fall behind today. The regression test covers all three file
types anyway, so one added later cannot rot unnoticed.
- `tickets/`, `docs-project/`, `prototypes/perf/project` and
`prototypes/worlds/project` were already clean.
