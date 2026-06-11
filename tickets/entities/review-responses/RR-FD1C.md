---
id: RR-FD1C
type: review-response
title: 'Round 1 #3: both buildSections branches need wiring; extract shared helper'
finding: |
  Plan only references the `properties`/`list` branch at sections.go L192-219. The `content`/`cards` branch at L279-308 is structurally identical and almost certainly the primary inline-edit target for TKT-IHC7C. AC 3's "the path that iterates entities rather than the entry" is ambiguous — both non-table branches iterate. The duplication is already a smell; adding two new fields to both branches makes it worse. Extract a shared helper while touching this.
severity: significant
status: addressed
resolution: |
  PLAN AC 3 rewritten: wire BOTH branches (L192-219 `properties`/`list` AND L279-308 `content`/`cards`). The shared work — copying typed properties, attaching the source entity reference — extracts into `buildSectionEntityData(ctx, e *entity.Entity, fields []ViewField, eDef *metamodel.EntityDef) SectionEntityData`. Both branches call it. The string-field building stays inline because it's per-section-config, not per-entity.

  Effort estimate stays `s`: the helper is ~20 lines; the call-site refactor is mechanical.
---
