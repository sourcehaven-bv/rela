---
id: BUGA-Y5OVNX
type: bug-analysis-checklist
title: 'Analysis: rela init writes a schema.yaml that rela migrate immediately flags as deprecated'
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

No special environment: default (fs) build, clean empty directory. `rela
validate` passed on the same file, so the schema was valid — just not current.

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

The four entity definitions in `DefaultMetamodelYAML()`
(`internal/metamodel/loader.go`) carried `id_prefix` but no `id_type`.
`ShortIDDefaultMigration.Detect` returns true for any entity missing `id_type`,
so the generated file matched the migration on its first read.

Confirmed `migrate` changed nothing else of substance: the only non-whitespace
diff it produced was inserting `id_type: sequential` into each of the four
entities.

## Fix Planning

- [x] Fix approach determined
- [x] Regression test planned
- [x] Related areas checked for similar issues

Fix: write `id_type: short` explicitly in the template. Deliberately `short`,
not the `sequential` the migration inserts — the migration exists to preserve
legacy behavior for projects predating the default change, whereas a NEW project
should get the current default. Verified a fresh init now mints `REQ-CGLP`.

Regression test asserts `migration.Detect` finds nothing on the output of the
real `Initialize`, against the whole registered migration set rather than this
one migration.

Related areas checked:
- `DefaultMetamodel()` (the Go-struct twin) leaves `IDType` empty, which
`GetIDType()` resolves to `short` — already consistent with the fix; verified by
a throwaway test comparing the two.
- `Initialize` writes only `schema.yaml`, so no acl.yaml/data-entry.yaml template
is affected.
- Docs mentioning `id_prefix` document the schema language generally; they are not
copies of the init template.
