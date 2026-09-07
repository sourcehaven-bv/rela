---
id: TKT-ZQV9O5
type: ticket
title: 'Expose current_user on where: surfaces (views, feeds, kanbans, CalDAV) and the CLI'
kind: enhancement
priority: medium
effort: m
status: backlog
---

## Problem

[[TKT-OIRBFH]] added `current_user` (with `is_current_user` /
`has_current_user`) to the predicate language and exposed it on ACL affordance
`when:` clauses and next-action `condition:`, with store pushdown. The `where:`
surfaces still cannot select per caller: view/list `where:`, feed `where:`,
kanbans and CalDAV collections all evaluate `internal/filter` clauses with no
principal awareness. The motivating case from the original ticket — a CalDAV "My
tasks" collection — is therefore still open.

## Proposal

Add a `condition:` key beside `where:` on the surfaces that take one (views,
lists, feeds, kanbans, `caldav:` collections), in predicate syntax, ANDed with
`where:`. Same two-key rule as next actions and automations: the dialects
overlap without erroring, so a sniffing heuristic would guess quietly.

```yaml
caldav:
  my-tasks:
    where: ["type = task", "status != done"]
    condition: "is_current_user(entity.assignee) or has_current_user(entity.watchers)"
```

- Compile once at config load with `Evaluator.CompileWithCurrentUser`; a
condition that does not compile is a load error.
- Stamp the identity once per request at the router boundary
(`predicatefns.ResolveQueryIdentity` + `WithQueryIdentity`, after
`resolvePrincipalEntity`), so every surface shares one derivation. The
next-action path currently derives it in a per-request scope binder supplied by
`appbuild` (`nextActionRequestScope`); converge on the boundary stamp and reduce
the binder to the agreement check. When doing so, compare `Tool` as well as
`ID()` in that check — today only the id is compared, which is fine while
nothing stamps upstream, but a boundary stamp with a divergent tool would bind
`current_user.tool` from the stamp (security review note on TKT-OIRBFH; `tool`
is diagnostic and never pushed, so no exposure today).
- Push `queryplan.ConditionPrefilters` into the list/feed query where the path
already goes through `visibleListByTypes`, and include the condition in
`StaticIndexSpecs` for the surfaces that are static.
- Fail closed everywhere: an unidentified request on a per-user surface is a
named error, never an empty or everyone's result.

## Design questions

- **Caching keyed on principal.** Anything memoizing filter results per
collection (CalDAV ctag, rendered feeds) must key on the identity once results
are principal-dependent; a cross-principal cache leaks one user's view to
another. Explicit test.
- **CLI `--filter`.** The CLI principal is `$USER`, not a graph identity. Either
resolve through the ACL policy's `principal_property` lookup when one exists, or
keep `current_user` undeclared on the CLI and say so.
- **Wizard forms.** The SPA's client-side condition engine passes
`current_user: {}`; decide whether the server supplies the identity to the form
context or the namespace stays server-only.
- **Free-text queries.** Next actions refuse a `condition:` on a free-text
query because the relevance cap runs before the condition. A view or feed
`where:` is not capped the same way; confirm before allowing the combination.

## Acceptance

1. `condition:` with `current_user` works identically on views, feeds and
`caldav:` collections.
2. Two principals hitting one CalDAV collection see different resources.
3. A zero/unknown principal on a per-user surface fails closed, with a test.
4. Any per-collection cache is keyed on the identity, with a test.
