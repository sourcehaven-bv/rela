---
id: BUG-N7QF2M
type: bug
title: Deleting an entity panics when comments are disabled (typed-nil subscriber in alias fanout)
description: |-
    DELETE of any entity panics with a nil pointer dereference when the `comments:`
    block is absent from the metamodel. Observed live on atlas (2026-09-16 06:57:30
    and 06:57:32 UTC), where commenting is off; each panic kills the serving
    goroutine and the delete fails.

    buildComments (appbuild/comments.go:37-40) signals "feature disabled" by
    returning a nil *comments.Service. That concrete pointer is passed straight into
    the variadic newAliasFanout(aliases, commentSvc) (appbuild.go:1763), which boxes
    it into an entitymanager.AliasRewriter interface carrying a non-nil type word.
    newAliasFanout's `if s != nil` filter (aliasfanout.go:38-42) does not see through
    that box, so the dead subscriber is kept. The fanout is then non-nil, which
    bypasses the Manager's `if m.deps.AliasRewriter == nil` fast path
    (entitymanager/alias_hook.go:67), and notifyAliasesOfDelete calls straight into
    the nil service: comments/service.go:185 dereferences s.store.

    EntityRenamed (service.go:169-174) has identical exposure; only its
    `oldID == newID` early return spares a no-op rename.

    buildStateAndAliases (appbuild.go:2182) returns a nil *caldavalias.Service the
    same way on a corrupt alias table, so the same crash is reachable by a second
    route.

    Fix (decided): reject boxed nils in newAliasFanout via reflection, so the
    documented "rewriter over the non-nil subscribers" contract holds for any
    subscriber rather than for the two current ones.
priority: high
why1: Deleting an entity dereferences a nil *comments.Service, because a disabled comment service is still registered as an alias-fanout subscriber.
why2: newAliasFanout filters with `s != nil`, which is false for a nil pointer boxed into an interface — the interface holds a type word, so it is not a nil interface.
why3: The "nil service IS the disabled signal" convention returns an untyped-looking nil that silently becomes typed the moment it crosses a variadic interface parameter, and the call site passes it unconditionally.
why4: The unit tests for the filter pass literal `nil` (comments_test.go:103-105), which the compiler converts to a true nil interface. That exercises a shape production never produces, so the guard looked covered while being untested against the real input.
why5: 'Systemic: "return nil to mean disabled" is used for several optional subsystems, but whether a nil survives depends on how each consumer receives it. There is no chokepoint that normalises a disabled subsystem, so every new consumer of an optional service re-decides the nil question and can reintroduce this crash.'
prevention: 'P2: normalise nil-ness at the wiring boundary — newAliasFanout drops any subscriber that is nil, including a typed nil, so no consumer depends on which nil shape it was handed. P4 (automated measure): regression tests that feed the REAL buildComments output (not a nil literal) into newAliasFanout and delete an entity with comments disabled.'
status: done
---

Fix implemented on branch `fix/alias-fanout-typed-nil`.

newAliasFanout now filters through isNilSubscriber, which unwraps the interface
with reflection and treats a nil pointer as absent. Reflection rather than a type
switch over the two known subscribers: the wiring pattern is the defect, and a
type switch would silently fail to cover a future optional subsystem.

Verified against the buggy version: TestAliasFanout_SkipsTypedNilBesideLiveSubscriber
and TestAliasFanout_DisabledCommentsSurviveDelete both reproduce the production
panic (invalid memory address or nil pointer dereference) before the fix and pass
after it. TestAliasFanout_NilWhenOnlyTypedNilsSubscribe pins the typed-nil case the
existing literal-nil test could not reach.

go test ./internal/appbuild/... ./internal/comments/... ./internal/entitymanager/...
green; golangci-lint over internal/appbuild reports 0 issues.
