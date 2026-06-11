---
id: TKT-B11W0
type: ticket
title: Enable contextcheck golangci-lint rule
kind: refactor
priority: medium
effort: m
status: done
---

Enable the `contextcheck` linter (currently commented out in `.golangci.yml`)
and fix the violations it surfaces.

Follow-up to TKT-AHUNF, which deferred contextcheck because enabling it required
a refactor that was out of scope for that lint-expansion ticket. A current run
(golangci-lint v2.11.4) reports **101 violations** (the stale comment in the
config says 84).

## Violation breakdown

Two distinct message shapes:

- **86× `should pass the context parameter`** — a ctx-aware caller invokes a helper that does not accept ctx, so the request context cannot be threaded down to the store/tracer/predicate call at the bottom. Fix = thread `ctx context.Context` through the private helper chains.
- **14× `Non-inherited new context`** — code in request scope creates a fresh `context.Background()` instead of inheriting the incoming request ctx. Fix = accept the incoming ctx and delete the `context.Background()` line.

## By package

| Package | Violations | Notes |
|---|---|---|
| internal/dataentry | 58 | api_v1.go (34, mostly `getEntity`/`outgoingRelations` helper chains), handlers_api.go (10), commands.go (5), affordances.go (3) |
| internal/mcp | 22 | prompts.go (11, mostly `_ context.Context` + `context.Background()` in prompt handlers), tools_*.go, resources.go |
| internal/analysis | 7 | analysis.go — `collectEntities` chain not ctx-threaded |
| internal/affordances | 5 | resolver.go — `passes`/`applyFieldGrants` chain ending in `prog.Eval(context.Background(), ...)` |
| internal/scheduler | 3 | scheduler.go + one test |
| internal/cli | 2 | validate.go |
| internal/{validation,script,lua} | 1 each | one is a test |

## Nature of the work

Mostly mechanical ctx threading. A partial migration already exists as
precedent: `App.outgoingRelationsCtx(ctx,...)` (ctx-aware) sits alongside the
legacy `App.outgoingRelations(id)` background-ctx wrapper; the work is to push
the Ctx variants up through the call chains and retire the
`context.Background()` wrappers. Several `context.Background()` calls are
genuine bugs where request cancellation cannot propagate (see RR-JWDHH, already
noted in review history).

## Acceptance

- `contextcheck` enabled in `.golangci.yml`, the explanatory comment block removed.
- `just lint` clean (no contextcheck violations, no new violations from other linters).
- No behavioral regressions — the threaded ctx must be the actual request/caller ctx, not a swapped-in `context.Background()`.
