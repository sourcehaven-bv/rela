---
id: TKT-IK889
type: ticket
title: Codify architectural learnings in CLAUDE.md
kind: docs
priority: medium
effort: xs
status: done
---

## Problem

A retrospective analysis of 31 done bugs, 659 review-responses, and ~100 done
tickets surfaced four recurring architectural lessons that are not yet codified
in `CLAUDE.md`. New code (and AI agents working in the codebase) keeps making
the same mistakes because the rules live only in tribal memory and prior PR
descriptions.

The lessons:

1. **Snapshot-once read protocol.** TKT-Z7HL → TKT-252Y → TKT-PYN1 → TKT-910WC
migrated 196 call sites to `workspace.Snapshot()`; despite that, new handlers
still call `ws.Meta()` repeatedly within one operation, which can observe
inconsistent state across reload boundaries.
2. **Don't leak storage/parsing types via return values.** Six remediation
tickets (TKT-021/022/033/034/270TQ/SNG55) removed `*markdown.Document`,
`*graph.Graph`, and `interface{}` returns that pulled implementation types into
every consumer. The interface-at-call-site rule already in CLAUDE.md doesn't
cover the return-type dual.
3. **Lock-upgrade dance ban.** The `App.mu` saga (TKT-WYYP, TKT-9NFK, TKT-PYN1,
TKT-252Y) replayed twice — once in workspace, once in dataentry. The pattern: a
single RWMutex tries to do reload coherency *and* write serialization; the fix
splits to `atomic.Pointer[State]` plus a separate `sync.Mutex`.
4. **Constructors must reject nil required fields.** Nine review findings
flagged `NewWriter`, `NewRouter`, `NewRootedFS`, `workspace.State` etc.
accepting invalid input and either panicking on first method call or silently
substituting a no-op.

## Scope

**In scope**

- Add the four rules to `CLAUDE.md` under the existing "Rules for new code"
and "Don't do this" sections.
- Keep additions terse — one paragraph each, matching the existing rule style.
- No code changes in this PR.

**Out of scope**

- Arch-lint enforcement of snapshot-once (sealing `Workspace.Graph`/`Meta`
private) — depends on TKT-7DJ2O handler migration.
- Custom analyzers for nil-rejecting constructors or leaky returns —
separate ticket if pursued.
- Project-FS helper / `filepath.Join(projectRoot, ...)` rule —
separate ticket (already TKT-92ID1).

## Acceptance criteria

- `CLAUDE.md` contains the four new rules.
- `markdownlint` passes on `CLAUDE.md`.
