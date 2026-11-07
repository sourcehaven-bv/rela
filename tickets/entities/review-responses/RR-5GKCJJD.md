---
id: RR-5GKCJJD
type: review-response
title: 'Code: disabled link segment announced no state'
finding: A disabled link option was a bare span with aria-disabled, which the generic role does not support; srHint was not rendered.
severity: minor
resolution: The span has role=link with aria-disabled and renders srHint.
status: addressed
---
