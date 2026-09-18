---
id: RR-U95ITB
type: review-response
title: 'BindRequest never called: identity scopes 500 on every page'
finding: 'QueryScopeResolver.BindRequest was declared on the dataentry seam, implemented in appbuild.QueryScopeResolver, and passed through adaptedQueryScopes — but no code ever called it. So applyScope -> Evaluate -> MatchesAs found no query identity on ctx and returned ErrNoCurrentUser, making every list/kanban/export/feed/gantt page of a type with an identity-bearing scope answer HTTP 500. The ticket''s own headline example, mijn: "is_current_user(entity.toegewezen_aan)", did not work at all. Found independently by both the code reviewer and the security reviewer.'
severity: critical
resolution: Wired the binder into the read path. viewQueryScope now returns a resolvedQueryScope struct carrying Scope, Props, Eval and Bind together, and scopedSortedEntitiesScoped calls bind(ctx) once per request before any row is evaluated (never per row — resolution reads the store). Added TestQueryScopes_IdentityScope, which asserts both that alice gets her own rows and that bob asking the same question does not get alice's.
status: addressed
---

## Why it survived until review

No test in the branch declared an identity scope. The e2e fixture had only
`default` and `archief`, both plain property comparisons. Compiling an identity
scope, resolving it, pushing its prefilters and applying it all look identical
whether or not the identity was ever bound — the difference appears only when
the program is actually evaluated against a row.

`internal/scopes` even carries a 28-line `RequiresIdentity` doc explaining that
an identity default makes pages error *on a deployment without identity*, and
`appbuild` emits a startup warning saying the same. Both were written on the
assumption that the with-identity case worked.

## The structural lesson

The four resolved pieces are one contract: the program was compiled by the
resolver that supplied `Eval`, and `Eval` can only evaluate it against an
identity `Bind` stamped. Returning them as four separate values is what let one
be silently dropped — the call site compiled fine without it.

They now travel as `resolvedQueryScope`. That does not make forgetting the bind
a compile error (the reviewer's suggestion of a handle that cannot be evaluated
unbound would), but it puts the binder where a reader of the struct cannot miss
that it exists. See [[IDEA-queryscope-bound-handle]] if the stronger form is
wanted.
