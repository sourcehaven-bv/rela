---
id: RR-2Z06QB
type: review-response
title: Unreachable check misfires when an earlier guard failed to compile
finding: 'v.guard returns nil on a compile error (session.go:576-591), so a compiled Transition with a broken `when` looks unguarded. If branchChecks reads the compiled When, every later covered entry gets a spurious "can never fire" on top of the real compile error. Fix: base the "has no when" test on raw.When != "" (the rawTransition), not t.When.'
severity: minor
resolution: The unreachable check now reads a `guarded` flag set from the raw `when:` text, with a test that a broken guard yields only its compile error.
status: addressed
---
