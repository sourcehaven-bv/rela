---
id: RR-NFPXU
type: review-response
title: Per-display-mode priority chain not stated
finding: 'Spell out: list/cards/content = list.detail_view -> /entity/:type/:id; table cell = cell.link -> list.detail_view -> /entity/:type/:id (with cell.link winning when present, preserving server-driven config).'
severity: minor
resolution: 'Per-mode priority chain documented in plan. With the entity_types config change, all consumers share: opts.cellLink -> entity_types.<type>.detail_view -> /entity/:type/:id.'
status: addressed
---
