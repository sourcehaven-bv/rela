---
id: RR-OYENHV
type: review-response
title: 'Plan understates the Go plumbing: span must cross a second DTO populated at two sites, and misses forms entirely'
finding: |-
    The plan lists the Go change as one field on dataentryconfig.ViewSectionField (config.go:574) plus a TS mirror. Tracing the actual path shows more.

    1. A SECOND DTO. The config struct is not what reaches the frontend. internal/dataentry/sections.go:56 defines a separate `SectionFieldData` (Property, Label, Values, PropType, Inaccessible) which is what the wire layer emits. Span must be added there too, then carried through the wire converter into the view JSON the SPA consumes.

    2. TWO POPULATION SITES. SectionFieldData is constructed in two places with near-identical field-by-field literals: sections.go:186 (buildSectionEntityData, for cards/list sections) and sections.go:228 (the entry-source 'properties' branch in buildSections). Adding Span to only one silently produces spans that work on the detail page but are dropped on card/list sections. These two blocks are already duplicated logic -- a good candidate to factor while touching them.

    3. FORMS ARE A DIFFERENT TYPE ENTIRELY. `FormField` (config.go:176) is a wholly separate struct from ViewSectionField, with its own 11 fields and its own validateFormField validator. The plan's span model therefore applies to VIEW sections but not to FORMS. The New Ticket form (/form/create_ticket) -- one of the sloppiest surfaces in the original tour, and the screen most analogous to the Filament reference, which is a FORM builder -- would keep its single-column-only layout with no way to group fields.

    That is a scope decision, not an oversight to silently absorb. Either add Span to FormField as well (doubling the config/validation/plumbing work but making the model coherent), or state explicitly in the ticket that forms are out of scope for span and say why. Shipping 'authored spans' that don't work on forms will read as a half-built feature.
severity: significant
resolution: 'All three sub-points absorbed into the plan. (1) SectionFieldData (sections.go:56) now explicitly carries Span. (2) Both construction sites are named in the ticket — sections.go:186 and :228 — with AC 8 requiring Span to survive to view JSON on BOTH, verified for a card/list section as well as the detail page; the duplicated literal blocks get factored while there. (3) Forms are IN scope: FormField (config.go:176) gains Span alongside ViewSectionField (config.go:574), since the Filament reference is itself a form builder and /form/create_ticket was among the sloppiest surfaces in the tour. Effort raised l -> xl to reflect the doubled config/validation/plumbing surface.'
status: addressed
---

Traced during design review:

- `internal/dataentry/sections.go:56` — the `SectionFieldData` DTO.
- `internal/dataentry/sections.go:186` and `:228` — the two construction sites.
- `internal/dataentryconfig/config.go:176` — `FormField`, disjoint from
`ViewSectionField` at `:574`.

The Filament reference the user supplied is a *form* builder, which makes the
forms gap especially visible.
