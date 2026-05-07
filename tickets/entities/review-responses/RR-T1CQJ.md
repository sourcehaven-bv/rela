---
id: RR-T1CQJ
type: review-response
title: validationErrorf surfaces only first ValidateEntity error
finding: |-
    api_v1.go:710 uses errs[0].Message when ValidateEntity returns multiple errors. Multi-field invalid PATCH shows one error; user fixes it, re-PATCHes, sees the next — fail-then-fail-again loop.

    Fix: concatenate or list all errors:
    ```go
    msgs := make([]string, len(errs))
    for i, e := range errs { msgs[i] = e.Message }
    return validationErrorf("validation: %s", strings.Join(msgs, "; "))
    ```
severity: nit
resolution: ValidateEntity error reporting now concatenates ALL errors via strings.Join(msgs, '; ') instead of surfacing only errs[0].Message. Multi-field invalid PATCH shows the full list.
status: addressed
---
