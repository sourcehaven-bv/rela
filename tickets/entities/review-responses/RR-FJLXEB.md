---
id: RR-FJLXEB
type: review-response
title: Typed-nil NopACL and ReadOnlyACL pointers missing from edge cases
finding: The old switches match nil pointers of these types as no-policy and read-only; ModeOf must not nil-check them.
severity: minor
resolution: TestModeOf_ClassifiesEveryShape covers value, pointer and nil-pointer forms of both.
status: addressed
---
