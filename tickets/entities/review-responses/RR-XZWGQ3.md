---
id: RR-XZWGQ3
type: review-response
title: Query scopes recompile on every list request
finding: 'viewQueryScope calls the resolver func per request, and appbuild.QueryScopes calls scopes.Compile(meta) plus predicatefns.NewEvaluator(meta) each time — so every list read recompiles every declared scope through a brand-new evaluator whose cache starts empty. Measured at ~17µs and 43KB per request for a three-scope schema, scaling with (types × scopes). Concurrency is fine: the evaluator guards its cache with a mutex and a compiled Program is immutable, so sharing is safe.'
severity: minor
reason: 'Deferred because it is the established pattern, not something this change introduced: appbuild.NextActionMatchers is built identically and called once per next-actions request, for the documented reason that config and metamodel both reload at runtime and a resolver captured at boot would serve a scope the operator has since edited. Fixing it properly means memoising on the *metamodel.Metamodel pointer (swapped wholesale on reload) for BOTH call sites, which is a change to the next-action path as much as this one. Doing it here would leave the two inconsistent again in the other direction. The list endpoint is hotter than the next-actions panel, so scopes make the existing cost more visible — that is the argument for fixing it, not for fixing it here.'
status: deferred
---

Note `appbuild.prepare` already compiles the scopes once at boot and discards
the result, using it only as a load-time gate. That compiled value is the
natural thing to memoise against, and it is already reached on the reload path.
