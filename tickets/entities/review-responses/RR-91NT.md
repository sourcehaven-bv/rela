---
id: RR-91NT
type: review-response
title: 'Toolbar button placement: ''between link and code'' breaks visual grouping'
finding: 'Current toolbar groups inline-formatting (link, code, quote) together. Inserting the entity-ref button between `link` and `code` splits the inline group. Better placement: a new group separator before the button, putting it visually adjacent to `link` but distinct. Tiny detail, but the existing toolbar''s visual rhythm is intentional (separator at `|` between heading-set, list-set, inline-set, etc.). Add `''|''` before the new button to keep groupings honest.'
severity: nit
resolution: 'Plan §Approach §2 + AC 1: button placed AFTER the inline group (''link'', ''code'', ''quote'') with its own ''|'' separator, preserving the toolbar''s existing visual grouping.'
status: addressed
---
