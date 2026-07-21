---
id: RR-MZFKO7
type: review-response
title: AC9's EveryoneGrants mirror is the wrong pattern and produces an actively misleading acl map report
finding: 'AC9''s mitigation is to report asserted grants globally mirroring EveryoneGrants (acl/access.go:74-80). That pattern does not transfer. EveryoneGrants is a statement of fact: the everyone role genuinely applies to every principal including unauthenticated ones, so reporting it globally is accurate. An asserted grant applies to exactly those principals whose token carries the mapped claim — an unknowable subset, because the population lives in the IdP, not the graph (enumeratePrincipals at aclmap/enumerate.go:41-85 draws from assignment keys, user-entity-type entities, membership edges and role-relation edges; none sees a JWT claim). Rendering an asserted grant in the same global slot tells an operator running `rela acl map` that EVERYONE holds that role. For a mapping like {admin: superuser} that is catastrophically misleading in exactly the moment the report is consulted — post-incident, answering ''who could have done this?'''
severity: significant
resolution: 'Accepted — the EveryoneGrants mirror is dropped. The reviewer''s distinction is correct and load-bearing: EveryoneGrants is a statement of fact (the everyone role genuinely does apply to every principal, so reporting it globally is accurate), whereas an asserted grant applies to an unknowable subset whose population lives in the IdP, not the graph (enumeratePrincipals at aclmap/enumerate.go:41-85 draws only from graph-visible sources). Reusing the global slot would tell an operator running `rela acl map` that EVERYONE holds the mapped role — worst possible answer at the worst possible moment, since the report is consulted post-incident to answer ''who could have done this?''. AC9 rewritten: emit a distinct, separately-labelled section (''conditional grants (asserted claims)'') naming claim -> role(s) and stating explicitly that holders are not enumerable from the graph. Does not reuse the Everyone struct or its wire field. Also pinning aclmap/whocan.go:141 by test: it constructs its Principal with no asserted roles, which is correct, so nobody later ''fixes'' enumeration by injecting synthetic roles.'
status: addressed
---

## Finding

AC9 mitigates `acl map` under-reporting by "report asserted grants once,
globally, mirroring `EveryoneGrants` (access.go:74-80)". The pattern does not
transfer.

`EveryoneGrants` is a **statement of fact**: the everyone role genuinely applies
to every principal, including unauthenticated ones. Reporting it globally is
*accurate*.

An asserted grant is the opposite — it applies to **exactly those principals
whose token carries the mapped claim**, an unknowable subset whose population
lives in the IdP, not the graph. `enumeratePrincipals`
(`aclmap/enumerate.go:41-85`) draws from assignment keys, user-entity-type
entities, membership edge sources and role-relation edge sources; none of them
can see a JWT claim.

Rendering an asserted grant in the same "global" slot tells an operator that
**everyone** holds that role. For `{admin: superuser}` that is catastrophically
misleading in precisely the moment the report is consulted — post-incident,
answering "who could have done this?"

## Resolution

Use a distinct, honestly-labelled section: *"conditional grants (asserted
claims): claim `admin` → role `superuser`; holders not enumerable from the
graph."* Same structural "no enumerable principal" problem, opposite semantics —
do not reuse the `Everyone` struct or its wire field.

Also worth an explicit test: `aclmap/whocan.go:141` constructs
`principal.Principal{User: user, Tool: principal.ToolCLI, RawUser: rawShown}`
with empty asserted roles. That is correct; pin it so nobody later "fixes" the
enumeration by injecting synthetic roles.
