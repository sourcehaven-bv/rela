---
id: RR-2FY0O8
type: review-response
title: Included files silently drop their automations
finding: 'partialMetamodel has no Automations field so automations in an include: are discarded without warning'
severity: significant
reason: 'Out of scope and genuinely separate: internal/metamodel/include.go''s partialMetamodel has no Automations field so automations in an included file are silently dropped. That is a metamodel-include bug with its own blast radius (silent non-execution) and needs its own ticket plus its own tests - folding it into an audit-attribution change would make both harder to review. Recorded here so it is not lost.'
status: deferred
---

Found while verifying the name-validation claim, and outside this diff's scope,
but it should not be lost: `internal/metamodel/include.go` does not merge
automations at all — `partialMetamodel` has no `Automations` field, so any
`automations:` block in an included file is **silently dropped**.

No error, no warning; the automations simply never run. Given that this ticket
is about operators being able to reason about which automation caused what, an
automation that silently does not exist is a sharper version of the same
problem.

Needs its own ticket rather than a fix here.
