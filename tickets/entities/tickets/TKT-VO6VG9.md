---
id: TKT-VO6VG9
type: ticket
title: Version tags
kind: enhancement
priority: medium
effort: l
started: "2026-10-07"
completed: "2026-10-08"
status: done
---

## Description

Named, movable tags on entity versions (postgres and sqlite). A tag names one
version of one entity face: "the version we sent to X", or the merge base of a
sync connector (`sync/basecamp`). TKT-SM20FG uses a tag as its 3-way merge base
instead of storing a version number in the entity, which would change the entity
and so make a new version.

## Rules

- A tag points at a version row, so it follows the entity through renames.
- Tags are per face, like versions.
- Tagging the current state captures a version synchronously when the sweep
has not yet captured that content.
- Namespaces: the default namespace may be moved by any principal with update
rights on the entity. Prefixed namespaces (`sync/`) are reserved for
operator-declared identities.
- Moving or deleting a tag is a write and is audited.
- Purging a tagged version is refused unless forced; a forced purge drops
the tag.
- Backends without versioning raise the same error as `rela.history`.

## Scope

In: tag table + store capability on pgstore and sqlitestore, conformance tests,
Lua (`rela.tag_version`, `rela.version_by_tag`, tags in `rela.history`), CLI.
Out: relation-version tags, history panel UI.
