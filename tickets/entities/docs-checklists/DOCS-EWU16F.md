---
id: DOCS-EWU16F
type: docs-checklist
title: 'Docs: visibility: new internal/visibility package — Reader (PolicyReader/AllowAllReader) + tracer decorator + conformance suite'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Package godoc carries the full contract: gate-first ordering, stored-type check, hidden=nonexistent semantics, read-out-only scope, capability-not-identity, fail-closed redactor contract, accepted residuals (FindPath timing, HasCycle-through-hidden)
- [x] Every exported type/function documented (Reader, PolicyReader, AllowAllReader, VisibleTracer, DeclarativeGate + Bind, PolicyRedactor, NopGate, NopRedactor, conformance suite)

## Project Documentation

- [x] CLAUDE.md package table: `internal/visibility` row added
- [x] CLAUDE.md "Rules for new code": read-out visibility-wrapper rule added (pointing at DEC-ZBI39P)
- [x] .go-arch-lint.yml component block documents the dependency rationale (why tracer, why NOT store)

## External / User-Facing Documentation

- [x] ~~docs/metamodel.md~~ (N/A: no metamodel change)
- [x] ~~docs/cli-reference.md~~ (N/A: no CLI change)
- [x] ~~docs/data-entry.md~~ (N/A: no data-entry behavior change in PR 1 — the package is deliberately unwired; user-facing docs land with PR 2 (TKT-L9Q669, export behavior) and PR 3 (TKT-ZF2DTV, Lua read semantics + release note))
- [x] ~~README.md~~ (N/A: internal package)
