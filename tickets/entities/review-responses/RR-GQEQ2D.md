---
id: RR-GQEQ2D
type: review-response
title: Hidden copy blocked only by Redacted with no wiring test
finding: taskItemValues expands from the raw source; only seen.IsRedacted stops a hidden value. The CopyReader contract does not require Redacted, production sets it only via the transitions.go change, and no wired test pins it. A stripping reader would leak hidden values with every test green.
severity: significant
resolution: Values now expand from the row as the caller sees it, so a reader that strips a hidden field cannot leak it even without naming it. The CopyReader contract now requires Redacted to name every hidden field whether or not it holds a value. TestCopyVisibility_NamesHiddenFieldsWithoutValues pins the wired reader from CompileTransitions with a policy that hides a field.
status: addressed
---
