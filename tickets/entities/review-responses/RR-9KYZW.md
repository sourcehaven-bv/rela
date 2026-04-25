---
id: RR-9KYZW
type: review-response
title: display_property table-row description is dense and duplicates the dedicated subsection
finding: 'GUIDE-metamodel.md (and derived docs/metamodel.md) crams a paragraph into the entity-types table cell for display_property. Most other rows are <80 chars. The same content also appears in the dedicated ''Display name'' subsection two paragraphs later. Fix: trim the table row to a one-liner (''Property name to use as display name; see Display name section below.'') and let the subsection do the work.'
severity: minor
resolution: 'Table row for display_property is now a one-liner: ''Property whose value names the entity. See [Display name](#display-name) below.'' All the rules and examples live only in the dedicated subsection. No more duplication. Also moved data-entry-specific framing (cards, breadcrumbs, etc.) to GUIDE-data-entry.md so the metamodel guide stays focused on the metamodel field.'
status: addressed
---
