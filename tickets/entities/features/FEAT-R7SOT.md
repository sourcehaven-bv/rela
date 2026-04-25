---
id: FEAT-R7SOT
type: feature
title: 'direction: incoming for RelationPicker form widget'
description: 'On data-entry edit forms, a `relations:` entry with `direction: incoming` and a non-cards widget (select/multi-select/search) must list the source entities linking TO the current entity, and allow adding/removing those reverse edges. Currently only the cards widget honors direction: incoming; the RelationPicker component ignores direction entirely, and GET /api/v1/{plural}/{id} only populates v1.Relations with outgoing edges, so the picker''s value list is always empty for incoming widgets.'
status: proposed
---
