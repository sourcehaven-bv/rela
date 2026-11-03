---
id: RR-J00JKZ
type: review-response
title: 'readsLeftOut is too coarse: it hides buttons that are decidable and loads own guards that can never hold'
finding: '`readsLeftOut` is true as soon as a guard names any left-out property (Approach 4). Take an earlier entry `message.decision == ''approve'' and message.note != ''''` and a compose preset with `decision: revise`. That guard can never take the post, but it joins `hidden`. When it admits every role that can click, the load refuses the button as unreachable (internal/realtimecfg/buttons.go:318-325), with a message that blames the earlier entry. In the other direction, an own guard such as `message.decision == ''approve'' and message.feedback != ''''` with preset `decision: revise` is skipped and loads a dead button. A sound improvement: evaluate the guard with the left-out fields bound to nil, and treat it as decided only when no left-out attribute was dereferenced (needs an access-tracking eval or a predicate partial-eval hook). Note that `GuardHolds` maps ErrNilOperand to false (internal/realtimecfg/guard.go:55-58), which is unsound here. A guard that compares a left-out field with nil must also stay undecidable. If this is deferred, document the over-hiding and the dead-button case, and add both as TestButtons_ComposeGuards cases so the behaviour is pinned.'
severity: significant
resolution: ADDRESSED Took the simple refusal instead of partial evaluation, which needs an access-tracking hook in internal/predicate (evalAttr binds an absent field to nil). The dead-button case is now refused. The over-refusal of an earlier guard that is in fact decided is documented with its workaround and pinned in TestButtons_ComposeGuards (AC 4, Risks).
status: addressed
---
