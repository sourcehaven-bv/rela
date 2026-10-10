---
id: TKT-RSVJKS
type: ticket
title: 'Per-face fields: policy'
kind: enhancement
priority: medium
status: backlog
description: 'fields: is keyed on type only, so a field restriction applies to every face at once. Allow ''type@face'' keys or entity.face in when: (GitHub #1760 part 2).'
---

## Description

`fields:` in `acl.yaml` is keyed on entity type
(`internal/affordances/validate.go`); a `type@face` key is a load error, and the
`when:` environment has no face (`internal/affordances/env.go`). A field rule
therefore holds for every face of a type.

Use case from #1760: a `document` with faces `concept` and `vastgesteld`. After
adoption one role may add a signed PDF to the `vastgesteld` face while the rest
of that face stays read-only.

## Options

- Accept `type@face` keys in `fields:`, as `read:`/`update:` already do.
- Expose `entity.face` in the `when:` environment.

The first matches the existing face grant syntax. Either must keep the redaction
side (`visible:`) consistent.
