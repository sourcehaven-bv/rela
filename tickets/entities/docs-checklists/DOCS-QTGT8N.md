---
id: DOCS-QTGT8N
type: docs-checklist
title: 'Documentation: per-automation audit attribution'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious — `triggeredByCtx`'s godoc is the main
one, and it earns its length: it states *both* guards (empty name, and
already-labelled ctx) and **why each is load-bearing**, because both were
re-derived incorrectly during this ticket. The empty-name guard exists because a
dangling `automation:` is worse than the generic label it replaces; the
already-labelled guard exists because tagging would overwrite an enclosing
`schedule:<task>` and silently shrink the answer to "what did last night's run
do?".
- [x] Function/type docs if public API — `automation.RelationToCreate` is new
and exported; its doc says why the name travels in a struct rather than a
parallel `[]string` (a parallel slice can drift in length or order with no
compile-time signal, and this value crosses a package boundary where the
consequence is a silently misattributed audit record).
`EntityToCreate.AutomationName` cross-references `LuaToExecute.AutomationName`,
the field it mirrors.

**Corrected, not just added.** Three comments asserted the pre-fix behaviour as
fact and would have misled the next reader:

- `cascadehost.go`'s `recordCascade` doc cited *"automation.Result doesn't carry
per-action names through the engine"* — the exact limitation this ticket
removes, pointing at the function whose new doc says the opposite.
- The `cascadeHost` type doc claimed records carry `triggered_by="automation"`.
- `TestAudit_UnnamedAutomationKeepsGenericLabel`'s comment described a test that
drives the runner directly and asserted the engine always supplies a name —
neither true.

## Project Documentation

- [x] ~~README updated~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated~~ (N/A: no new pattern. The composition policy this
ticket settled is documented where operators read it — the audit-log guide —
rather than in the contributor rules, because it describes the *log's*
semantics, not a coding convention.)
- [x] ~~Help text accurate~~ (N/A: no CLI change — no new flag or command)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: this repo has no CHANGELOG; releases are
cut from commit history — see `docs/releasing.md`)
- [x] API docs updated — `docs-project/entities/guides/GUIDE-audit-log.md`
(source of truth) → `docs/audit-log.md` (generated via
`./scripts/generate-docs.sh`; never edited directly). Three changes:

  1. **Removed the known-gap section.** "Per-automation attribution for cascaded
relations / entities" documented exactly the limitation this ticket removes.
  2. **Corrected the `triggered_by` label list.** It described
`automation:<name>` as scripted-only and `automation` as "generic label by
design" — both now false. It also briefly claimed *"you should not normally see
it, since every automation declared in `schema.yaml` has a `name:`"*, which
review correctly rejected: `name:` is an ordinary optional field on an
`automations:` list entry and **no loader validation requires it**. Documenting
an unenforced invariant as a guarantee would send the next operator on a
false-positive hunt, so the text now says what the label actually means.
  3. **Added the composition policy.** "One label per record: the outermost
cause wins" — with the scheduler example, because that is the query the policy
protects.

Regeneration touched only `docs/audit-log.md`, confirming no unrelated drift.
