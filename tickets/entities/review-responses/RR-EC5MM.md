---
id: RR-EC5MM
type: review-response
title: Status code distinction for malformed-vs-validation is muddled
finding: |-
    AC #9: 'missing type field returns 422.' But missing required field on JSON document is conventionally 400 (malformed request body), not 422 (server understood but rejected meaning). Plan is inconsistent: malformed JSON → 400, data: null → 400, but missing required scalar → 422.

    Fix: pick one rule and apply consistently:
    - Errors detected while parsing/validating request shape (JSON validity, required scalar fields, type fields on resource identifiers) → 400 with structured pointer ({pointer: '/relations/tagged/data/0/type', message: 'type field required'}).
    - Errors detected while validating against metamodel (unknown relation type, unknown target ID, target type mismatch, meta property type mismatch) → 422.

    Update ACs: #9 (missing type) → 400. #6, #7, #8, #10 stay 422.
severity: significant
resolution: 'Decision #9: shape errors (malformed JSON, missing required scalar, missing type field) → 400 with structured pointer. Metamodel validation errors → 422. AC #12 (missing type) → 400; ACs #9-#11, #13 → 422. Consistent rule applied throughout.'
status: addressed
---
