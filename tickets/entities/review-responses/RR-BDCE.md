---
id: RR-BDCE
type: review-response
title: Non-entry content sections render fake-interactive checkboxes
finding: renderMarkdown is called from three template sites in EntityDetail.vue. Only the entry section has @click="contentClick" wired. The two other v-html blocks (content-block at line 498-500, content-cards at line 514) now emit enabled data-cb-idx-tagged inputs without a wired-up handler. Users will see clickable checkboxes that silently no-op — the same failure mode this PR fixes, relocated to a different surface.
severity: significant
resolution: 'Added `interactive` flag to `renderMarkdown` (default false). Non-entry call sites in EntityDetail.vue (content-block at line 498-500, content-cards at line 514) now render checkboxes with marked''s default `disabled` attribute — visibly inert. Only the entry-content call site passes `interactive: true`, which is the only place with `@click="contentClick"` wired up. Unit test added asserting non-interactive renders contain `disabled` and no `data-cb-idx`.'
status: addressed
---
