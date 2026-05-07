---
id: RR-7PJY2
type: review-response
title: Table cells already have server-resolved cell.link — client helper must not override it
finding: 'Table cell <a v-if="cell.link" :href="cell.link"> already gets the correct href from server''s resolveLinkTarget (sections.go:217), which honors per-column ''link:'' config and document/* mappings. If the click handler now builds the URL client-side via the helper, we have two sources of truth that can disagree. Specify: client helper only runs when cell.link is empty (server''s column-link override wins). Otherwise we silently change behavior of existing per-column link configs.'
severity: significant
resolution: entityDetailHref takes opts.cellLink and returns it verbatim when present. Server-resolved per-column links retain their behavior; client helper only fills the gap when the server didn't resolve a link.
status: addressed
---
