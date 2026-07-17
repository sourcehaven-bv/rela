---
id: RR-XC2NTO
type: review-response
title: Duplicate principal rows when a user is both a resolvable assignment key and its own entity
finding: 'With principal_property configured, enumeratePrincipals yields both the raw key (jv@corp.com) and the resolved entity (PERS-JV). accessFor(raw) resolves to PERS-JV (Raw=jv@corp.com); accessFor(PERS-JV) resolves to '''' and emits PERS-JV (Raw=''''). Both have Principal==PERS-JV with identical routes; WhoCan appends without dedup, so the same human appears twice. Corrupts the drift/diff artifact this feature is building toward. Fix: merge candidates by effective principal (map[effectivePrincipal]->unionedRoutes) before emitting rows.'
severity: significant
resolution: WhoCan now merges candidates by effective principal into a map[principal]->PrincipalAccess, unioning routes (mergeRoutes, dedup by Route value) and preferring a populated Raw. A human reachable via both a raw key and its resolved entity collapses to one row. Covered by TestWhoCan_PrincipalPropertyResolution (asserts exactly one PERS-ALICE row).
status: addressed
---
