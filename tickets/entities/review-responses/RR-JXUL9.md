---
id: RR-JXUL9
type: review-response
title: Migration silently inherits detail_view to sub-lists that previously had none
finding: 'tickets/data-entry.yaml: idea has 3 lists (all_ideas with detail_view, active_ideas without, game_changers without). After migration, all three inherit idea_detail because the SPA falls back to entity_types. This is a behavior change for sub-lists. Either document as expected (probably correct: subset views want the same detail page), or migration writes detail_view: '''' to the previously-no-detail lists to preserve old behavior. Decision needed.'
severity: significant
resolution: 'Decision documented in plan: accept inheritance to sub-lists as expected behavior (subset/filter views should send users to the same detail page as the canonical list). Migration package-doc comment will explain. Verified against tickets/data-entry.yaml: 3-list idea group, 1-list future-concept/feature/concept groups -- inheritance is the desired behavior.'
status: addressed
---
