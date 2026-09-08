---
id: DOCS-IRYGXC
type: docs-checklist
title: 'Documentation: current_user in the predicate language and next-action pushdown'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Exported symbols have godoc
- [x] Non-obvious decisions explained at the call site

The comments carry the *why*: `CurrentUserType` says why it is a record and not
a string (operator `acl.yaml` passes it whole to `has_role`); `ErrNoCurrentUser`
says why absence is an error and not an empty binding (an empty identity would
match every unset property); `DeclareCurrentUser` says why it is separate from
`Declare` (validation, scheduler and index derivation must keep it a compile
error); `Program.ConstEqualities` states the three soundness restrictions and
why each exists; `PrefilterSpec.ConstFuncs` names the caller's assertion the
engine cannot verify; `ConditionPrefilterer` documents the raw-vs-redacted
asymmetry and why it is sound; `nextActionRequestScope` explains stamp/principal
agreement and names the ACL precedent; `bindingContext.identity` explains why
the attribution placeholder is not a user; `store.PropEqual` names the
list-membership reading a non-scalar equality has.

## Project Documentation

- [x] `docs/data-entry.md` — new "Per-user sources: `current_user`" subsection
under the next-action `condition` reference: the three spellings, what
`current_user.id` resolves to with and without a `user_entity_type`, the
fail-closed refusal (`next_action_identity_required`), and what is pushed to the
store versus evaluated in Go (including that membership is pushed but not
indexed).
- [x] Limitations documented, not just capabilities

Stated limitations: `current_user.tool` is diagnostic only; anything under
`or`/`not` or a typed comparison stays Go-side; a condition on a free-text query
is refused at load; `where:` surfaces and the CLI are not covered
([[TKT-ZQV9O5]]).

## External Documentation

- [x] ~~README updated~~ (N/A: no new binary, flag or command; a config-language
addition documented in the data-entry reference.)
- [x] ~~CHANGELOG~~ (N/A: the repo does not keep one; the ticket and commits
carry the history.)

## Cross-references

- [x] `CLAUDE.md` — the "Derived static-query indexes" rule now states that a
next-action `condition:` participates in both pushdown and index inference,
which shapes are pushed, and why list membership derives no index (a GIN shape
needs its own EXPLAIN test first). Added because the rule as written would
otherwise be read as "query filters only".
- [x] Field comment on `dataentryconfig.NextActionSource.Condition` updated so
the select/refine split is described as a LANGUAGE split, not a "where work
happens" split, and names the identity requirement.
