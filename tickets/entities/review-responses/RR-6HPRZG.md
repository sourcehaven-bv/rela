---
id: RR-6HPRZG
type: review-response
title: Required-property key moves to a YAML path that does not exist
finding: 'Today a missing required property is reported at `...post.props` (reminders_test.go:161 and load_test.go:445). The plan reports it at `...post.props.<name>`, which is a key the operator never wrote. Nothing maps Problem.Key to a YAML node today (load.go only prints key: reason), so this is a choice, not a bug. Other surfaces in the package differ: buttonChecker.checkLeftOut reports a missing required property at the preset key, not at a per-property key. State the decision in the plan and give the reason. Either keep `*PropError` for the undeclared and value refusals only and report the missing required property at `key` (this matches checkLeftOut and the current key), or accept the new key and document it as the convention.'
severity: minor
resolution: "addressed\tDecided to keep a missing required property at ...post.props, matching today and checkLeftOut; it stays a plain error, and propError covers only undeclared and value refusals."
status: addressed
---
