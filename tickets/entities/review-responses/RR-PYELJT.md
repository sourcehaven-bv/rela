---
id: RR-PYELJT
type: review-response
title: acl map reports everyone and asserted grants past the client ceiling
finding: typeBaseline added everyone/asserted routes from the policy-wide Declarative.EveryoneGrants/AssertedGrants, which read policy.Roles without the ceiling. --webhook and --as therefore reported reads the ceiling removes, and --expect would attest them.
severity: critical
resolution: Added Request.ClampedEveryoneGrants and Request.ClampedAssertedGrants (ceiling first, then roleFor). typeBaseline uses them. Asserted routes are skipped for webhook principals, which never carry claims. Tests cover --as and --webhook.
status: addressed
---
