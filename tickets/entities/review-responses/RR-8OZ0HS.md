---
id: RR-8OZ0HS
type: review-response
title: Typed-nil regression is pinned in aclaudit, not in the CLI where the bug actually was
finding: internal/cli/acl.go:65-74 — the typed-nil handling is correct (`var perms aclaudit.PermissionConsumer` declared at interface type, never assigned on the error path, so it stays a true nil interface and Audit's `perms == nil` fires). But it is untested at the call site. TestAudit_A7_TypedNilConsumerIsAnAnswer lives in internal/aclaudit and its comment claims it 'pins the distinction the CLI's typed-nil trap violated' — it does not. It pins the CALLEE's behaviour when handed a typed nil. The bug was in the CALLER, and the caller has zero tests. Rewriting the wiring to the obvious-but-wrong form (`perms := loaded; if err != nil { perms = nil }`, which produces a non-nil interface wrapping a nil pointer and makes A7 run blind) leaves every test on this branch passing. That is precisely the regression these commits exist to prevent, and nothing catches it. I hit this exact defect during implementation and it was caught only by running the real binary — which means it will not be caught next time either.
severity: significant
status: open
---

## Fix

Extract the nil-vs-consumer decision into a testable package-private function
returning the interface type, and assert on the error path that the returned
interface is nil — including a `reflect.ValueOf(got).IsNil()`-style check that
catches the typed-nil form explicitly, since `got == nil` alone would pass for a
correct implementation but so would a wrong one if the helper returned the
concrete type.
