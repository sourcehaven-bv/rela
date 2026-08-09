---
id: RR-AN1J4V
type: review-response
title: 'span: 6.5 silently truncates to 6, the exact failure the loud validation targets'
finding: |-
    yaml.v3 accepts a float into an int field without error, so `span: 6.5` becomes 6 with no diagnostic. (By contrast `span: half` and `span: true` ARE properly rejected at unmarshal.)

    validateSpan cannot catch this -- it only ever receives the already-truncated int, by which point the information is gone.

    An author writing 6.5 (say, reaching for half of a third) gets a layout that quietly ignores what they wrote. That is verbatim the failure mode validateSpan's own doc comment says strict validation exists to prevent, so leaving it contradicts the rationale the rest of the design rests on.

    Catching it needs the raw YAML node -- either a custom UnmarshalYAML on the field, or checking the decoded document for a non-integer scalar at that key before the struct unmarshal discards it.
severity: significant
resolution: |-
    Fixed in 2ff8e0db with a named `Span int` type carrying a strict UnmarshalYAML. It decodes as a float first and errors when the value is not whole (`span must be a whole number of columns, got 6.5`), falling back to the plain int decode so a string or bool still produces yaml's own familiar message rather than an invented one.

    This has to live at the unmarshal boundary: validateSpan only ever sees an int, by which point the fraction is already gone.

    Pinned by TestSpanRejectsFractional across six cases -- 6 and 0 decode, 6.0 is accepted as whole, 6.5 is rejected, and `half`/`true` still produce the unmarshal error. Confirmed the pre-fix behaviour first with a throwaway probe (6.5 -> Span=6, err=nil) so the fix is answering a real observation rather than a suspicion.

    SectionFieldData keeps a plain int: the named type exists to enforce strict decoding at config load, and the wire DTO should not drag a config type onto the API surface.
status: addressed
---
