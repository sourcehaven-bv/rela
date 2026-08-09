---
id: RR-U1B4UN
type: review-response
title: span on a relation field validates server-side but is silently ignored
finding: |-
    FormFieldOrRelation.span (types/config.ts:145) is typed as valid on relation entries, and Go's validateFormField accepts a span on every form field including relations. But FormFieldList only forwards span to FieldRenderer -> FieldShell. RelationCards and RelationPicker never read it.

    Verified: grep for `field.span` / `props.span` / `span?:` across RelationCards.vue, RelationPicker.vue and FormFieldList.vue returns nothing.

    So `span: 6` on a relation field passes config validation, produces no error anywhere, and has no effect. Silently ignoring a value the server just validated is the worst failure mode of the three the review found -- the author concludes the feature is broken rather than that they used it wrong.

    Note RelationPicker's root carries `class="form-field relation-picker"`, so it DOES land in the grid at 12 columns via `.form-fields > *`; it simply can never be narrower. RelationCards is in the same position.

    Two defensible fixes: plumb span through both components, or reject a span on relation fields in Go with a specific message. The second is smaller and arguably more honest -- relation widgets (a card list, a multi-select) have a natural minimum width that a 3-column slot would break.
severity: critical
resolution: |-
    Fixed in 2ff8e0db by rejecting rather than plumbing.

    The review offered both options. Rejecting is the more honest one: RelationCards and RelationPicker render a card list and a searchable multi-select, which have a natural minimum width that a 3-column slot would break. Supporting span there would mean supporting a layout that looks wrong.

    FormRelation gains a Span field captured ONLY so validateFormRelation can reject it -- verified first that yaml.v3 silently drops an unknown key on that struct rather than erroring, so without the field the value would keep vanishing without a word. Message: `form %q: relation[%d] cannot have a span (relation widgets always take the full row)`. Pinned by TestRelationSpanIsRejected.

    The TS type keeps span on FormFieldOrRelation (it is a union, and span is valid on the field half) but now documents that it is field-only and why.
status: addressed
---
