---
id: TKT-K7HR49Y
type: ticket
title: 'Declarative mapped import: rela import --mapping (JSON/XML/YAML/CSV, diff, Lua hook)'
kind: enhancement
priority: medium
effort: l
started: "2026-11-14"
completed: "2026-11-14"
status: done
---

## Description

A generic, declarative importer. An operator-authored mapping file
(imports/<name>.yaml) turns external structured data (JSON, XML, YAML, CSV) into
entities and relations. An optional pure Lua hook covers what the mapping cannot
express. A diff is shown before anything is applied (--dry-run, -o json).
Motivating case: OSCAL control catalogs (ISO 27002, BIO2) into GRC projects; the
OSCAL mapping itself ships with the GRC demo projects, rela carries the engine
plus a small OSCAL example in tests and docs.

## Acceptance

- rela import --mapping <name> <file> [--dry-run] prints creates, per-property updates, relation adds/removes, entities missing from the source and per-record errors.
- Apply writes through entitymanager (create, PatchEntity, relations); validation, audit and ACL apply.
- Re-import of the same file is a no-op.
- XML is decoded with safexml (no XXE, caps); input size caps for every format.
- rela validate checks imports/*.yaml.
- Tests: JSON, XML, relation by key, idempotent re-import, dry-run writes nothing, Lua hook, ACL-denied write, malicious XML, OSCAL example.
