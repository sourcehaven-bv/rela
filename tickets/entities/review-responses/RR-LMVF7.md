---
id: RR-LMVF7
type: review-response
title: propsValueEqual uses fmt.Sprintf string-comparison — false positives across types
finding: |-
    api_v1.go:816-822: `return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)`. Verified misclassifications:
    - int(5) == "5" returns true (FALSE POSITIVE; dangerous combined with C3)
    - nested map ordering inside slices not guaranteed equal
    - The comment 'just produces false negatives' is wrong — it produces false positives too.

    Fix: use `reflect.DeepEqual` everywhere:
    ```go
    func propsValueEqual(a, b interface{}) bool { return reflect.DeepEqual(a, b) }
    ```

    Handles maps, slices, nested structures correctly. Crosses no type boundaries (int(5) != "5").
severity: critical
resolution: 'propsValueEqual replaced with valueEqual (uses reflect.DeepEqual + numeric normalization for int/float64 cross-type cases). relationsEqual rebuilt around valueEqual. Type boundaries respected: int(5) is NOT equal to ''5''. Numeric types (int, int64, float64) are compared via float64 conversion since JSON unmarshal produces float64 but disk-loaded YAML can produce either.'
status: addressed
---
