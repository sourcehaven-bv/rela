---
id: TKT-6OMC
type: ticket
title: Extract automation.Runner with consumer-side Host interface
kind: refactor
priority: high
effort: m
status: review
---

## Summary

Extract the automation-result-dispatch logic currently embedded in
`internal/workspace/workspace.go` (lines 1049–1202:
`applyAutomationSideEffects`, `processEntityCreations`,
`runCreatedEntityAutomation`, `applyRelationCreations`, `executeLuaActions`)
into a new `automation.Runner` service. Apply the consumer-side-interface
pattern (CLAUDE.md, "Consumer-side interfaces for callbacks and cycles") so
Runner does not hold a back-reference to whatever invokes it.

## In scope

- New `internal/automation/runner.go` with `Runner` type and consumer-side `Host` interface (3–4 methods: `GetEntity`, `CreateEntity`, `CreateRelation`, possibly more).
- `Runner.Process(ctx, host Host, ev WriteEvent) (Result, error)` — Host passed per-call, not at construction.
- Workspace's existing automation-dispatch code moves into Runner, parameterized over Host.
- `Workspace` (during the transition) implements Host directly and passes itself when invoking Runner.
- Cascade depth limit (`maxAutomationDepth = 50`) moves with the orchestration code.
- `triggered_by` ctx-wrapping (when audit lands) localizes inside Runner.

## Out of scope

- Building EntityManager as a real implementation that satisfies Host. That happens after this lands.
- Moving the rule-evaluation `automation.Engine` itself; it stays as the pure decision-maker that Runner wraps.
- Anything related to script.Runtime structure changes.

## Why

This is the prerequisite refactor for decomposing Workspace. Today the cascade
orchestrator is entangled with Workspace internals; without lifting it out into
a service with its own Host contract, every subsequent migration step has to
drag Workspace along.

## Risks

- Runner needs to recurse — `create_entity` actions can themselves trigger more automations. Verify that passing Host per-call still works for cascades; the recursion is `host.CreateEntity → m.CreateEntity → m.runner.Process(host, ...) → host.CreateEntity → ...` which is already what Workspace does internally.
- The Lua execution path within cascades must continue to work via the existing `script.Runtime` ; nothing in this ticket changes Lua semantics.
- Tests for automation cascades currently run against Workspace; after this they should be runnable against Runner with a stub Host. Verify a sample test migrates cleanly before committing to the new shape.
