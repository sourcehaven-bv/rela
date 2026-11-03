---
id: RR-P7BVJ3
type: review-response
title: Plan omits existing trigger tests whose expected key and reason change
finding: 'load_test.go "trigger prop enum value" (line 441) expects the reason `"extreme" is not valid`. Under the plan, the kind policy passes an enum and NormalizeProps answers `property "severity": must be one of [...]`, so this case fails. "trigger required prop" (line 445) expects key `triggers.breach-escalated.post.props` with `required property "summary"`. Under the plan, the key becomes `...post.props.summary` and the reason becomes `property "summary" is required`, so both the key and the substring fail. The Files and AC 3 sections name only "trigger prop over max" for load_test.go. Moving the required-property problem from the props key to a per-property key is a behaviour change for triggers that the plan states only for reminders. List both cases in AC 3 and the test table with their new key and reason, so the implementer does not choose them while fixing red tests.'
severity: significant
resolution: "addressed\tAC 3 and the test table now list \"trigger prop enum value\" (must be one of) and \"trigger required prop\" (key stays ...post.props, reason property \"summary\" is required) with their new reasons."
status: addressed
---
