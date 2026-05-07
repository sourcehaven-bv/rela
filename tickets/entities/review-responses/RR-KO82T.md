---
id: RR-KO82T
type: review-response
title: validationError.status field is dead weight (always 422)
finding: |-
    api_v1.go:790-799: type validationError has a `status int` field set in exactly one place (validationErrorf) to exactly one value (StatusUnprocessableEntity). 400 (shape error) path returns from the handler before WithTx, never going through this type. Field is unused; either drop it and hardcode 422 at the use site, or repurpose for shapeErrorf with 400.

    Fix: drop the status field; hardcode 422 in the writeV1Error call.
severity: minor
resolution: 'validationError.status is now genuinely used: 404 (entity not found inside tx) and 412 (ETag mismatch) construct it directly with non-422 status. validationErrorf still defaults to 422. The dispatch in the error response writes the appropriate code/title per status. No longer dead weight.'
status: addressed
---
