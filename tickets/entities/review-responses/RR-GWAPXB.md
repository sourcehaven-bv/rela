---
id: RR-GWAPXB
type: review-response
title: Edit/delete affordances duplicate the operation's authorization logic
finding: 'internal/realtime/msgops.go:98-121 (mayEdit, mayDeleteOwn) re-implements the checks of Service.Edit (ops.go:52-86) and call.deleteOwn (ops.go:173-183) by hand. The only purpose is to reuse cached gate answers. The godoc says the affordance and the operation ''agree'', but nothing enforces that. The next person who adds a check to Edit (a new state, a per-type rule) will not know mayEdit exists, and the SPA will then show a button that 403s, or hide one that would work. releaseCheck and actionCheck already show the right pattern: one check function used by both the operation and the affordance. Fix: extract `func (c *call) editCheck(m channels.Message) error` (with the gate answer injected or memoized on call) and call it from both Edit and messageActions. Make deleteOwn take the cached gate answer instead of keeping a second copy. Add a differential test: for every message in a fixture channel and every principal, each offered op succeeds and each op that is not offered is refused.'
severity: significant
resolution: Edit, delete_own, remove and release affordances now call the same check functions as the operations (editCheck, deleteOwnGate, moderator, releaseCheck) with memoized gates, and TestMessageActions_AffordanceAndOperationAgree compares affordance and operation outcome.
status: addressed
---
