---
id: TKT-6RSPA2
type: ticket
title: 'Guide: YAML anchors and merge keys for config reuse'
kind: docs
priority: low
effort: s
status: backlog
---

## Description

Document **YAML anchors and merge keys** as the supported way to reuse config
across blocks in rela project files.

Came out of TKT-IG54YO (calendar views): `calendars:` and `feeds:` declare the
same entity-to-event source fields. Rather than couple them with a shared Go
type, the decision was to keep the two independent and let operators share a
declaration with a YAML anchor.

`gopkg.in/yaml.v3` resolves anchors and merge keys at parse time, so this works
today with no code change — it is simply undocumented.

```yaml
_task_events: &task_events
  - entity_type: task
    where: ["status != done"]
    date: due_date

feeds:
  tasks:
    sources: *task_events
calendars:
  schedule:
    sources: *task_events
```

## Why this deserves its own guide

The pattern is not calendar-specific. Anywhere two config blocks accept a
similar shape — filters, filter controls, column sets, form field groups — an
operator may want one declaration. Documenting it once is better than mentioning
it in each block's reference.

It also carries a **design message**: rela prefers independent config schemas
that can diverge over shared types that lock the project in. When a block
changes incompatibly, users copy a bit of config. The guide should say so.

## Scope

- A guide covering anchors (`&name` / `*name`) and merge keys (`<<: *name`)
- Verify and state the limits honestly: anchors are **per-file** only, and
give textual reuse, not validated consistency — if two blocks' accepted fields
drift, an anchor satisfying one may fail the other at load time
- A worked example using real rela config
- Cross-references from the config reference sections that most invite reuse

## Acceptance criteria

1. A guide exists explaining anchors and merge keys against real rela config.
2. The per-file limitation and the no-validated-consistency caveat are stated.
3. The examples are verified to actually load.
4. The design rationale (independent schemas, divergence stays cheap) is recorded.
