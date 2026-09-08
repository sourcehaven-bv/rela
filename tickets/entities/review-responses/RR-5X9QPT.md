---
id: RR-5X9QPT
type: review-response
title: layered dropped the optional Subscriber capability silently
finding: Subscriber is an optional interface consumers type-assert for (dataentry/watcher.go:147), and FSLoader implements it. Wrapping an FSLoader in NewLayered made the assertion fail, silently disabling live config reload — no compile error, no test failure, just an operator editing data-entry.yaml and seeing nothing happen. That directly undercuts the disk-first premise, which exists so an operator's edit is the one that takes effect.
severity: significant
resolution: layered now implements Subscribe, forwarding to the first layer that satisfies Subscriber (primary first) and returning an explanatory error when neither does. Added TestLayered_Subscribe_ForwardsToCapableLayer covering both the no-capable-layer and forwarded cases.
status: addressed
---
