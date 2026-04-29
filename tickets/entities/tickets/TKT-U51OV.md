---
id: TKT-U51OV
type: ticket
title: Clarify action params vs entity global in data-entry docs
kind: docs
priority: low
status: done
---

## Description

The data-entry actions documentation showed a misleading example using
`rela.params["entity_id"]` and `rela.params["entity_type"]` to read the selected
row. In reality `rela.params` only holds the static `params:` map from
`data-entry.yaml`; the runtime selected entity is exposed as the Lua global
`entity` (set by the action handler when the request body provides `entity_id`).
A user copying the example would get `nil` for `entity_id`/`entity_type` and
silent failures.

## Changes

- Replaced the misleading example in `GUIDE-data-entry.md` with one that nil-checks the `entity` global and reads a real static param.
- Added an "Inputs available to the script" table distinguishing static config (`rela.params`) from the runtime selected-row entity (`entity` global, list-only).
- Documented the `entity` table shape (`id`, `type`, `properties`, `content`, `mod_time`, plus `prop()` and `strip_prefix()` methods).
- Clarified that `params` values must be strings (quote them in YAML).
- Noted the global write lock held during the 5-second action timeout window.
- Tightened the `rela.params` row in `GUIDE-lua-scripting.md`.

## PR

https://github.com/sourcehaven-bv/rela/pull/622
