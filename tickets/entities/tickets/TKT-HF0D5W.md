---
id: TKT-HF0D5W
type: ticket
title: Configurable backlinks section with per-type columns
kind: enhancement
priority: medium
effort: m
started: "2026-10-08"
completed: "2026-10-09"
status: done
---

## Description

Let an operator place the "Mentioned by" backlinks on an entity page through
`data-entry.yaml`, choose how they display, and define what each source type
shows. Today the panel is hard-wired after the documents panel on every type
(TKT-Q6AKUN).

Backlinks are heterogeneous: sources of any type can mention an entity. A single
`columns:` list fits that poorly, so sections gain per-type field and column
definitions, following the precedent of `nested` sections'
`parent_columns`/`child_columns`.
