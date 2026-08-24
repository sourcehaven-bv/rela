---
id: RR-4JWU2I
type: review-response
title: validate.go godoc claimed abort-before-any-output; header prints first in JSON mode
finding: runCardinalityCheck's new godoc said a store error 'aborts the check before any output is written', but the pre-existing `fmt.Println("\nChecking cardinality constraints...")` banner fires on !quiet regardless of output format — including --format json — before the error return. The comment documented a guarantee the code does not make; a reader would wrongly trust the JSON stream is clean on the error path.
severity: significant
resolution: 'Corrected the comment to state what is true: the error aborts before any VIOLATION output; the banner is the pre-existing plain-stdout header every validate check prints (properties/validations share the wart). Changing the banner''s format-awareness for cardinality alone would diverge from its sibling checks — left as the documented pre-existing shape rather than a one-check special case.'
status: addressed
---
