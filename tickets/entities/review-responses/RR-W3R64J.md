---
id: RR-W3R64J
type: review-response
title: The cascade host writes relations below the manager, so the check does not cover it
finding: 'cascadehost.go WriteRelation calls h.deps.Store.CreateRelation directly, bypassing Manager.CreateRelation and therefore requireRelationFaceFor. The reviewer raised this as an incoherence: an automation cascade could mint exactly the zero-tail content-scoped edge the manager refuses, and once minted the Lua/CLI/MCP paths could not address it. The doc comment also opened with ''name exactly what the metamodel declares, never more, never less'', which overclaimed once the function was made one-sided.'
severity: minor
resolution: 'The premise dissolved with RR-HQUW7V: the cascade host passes NO FromFace at all (verified at the call site), so it only ever writes the zero tail — which is now legal rather than refused. There is no longer a coordinate the cascade can mint that the manager rejects. Documented anyway at the declaration, since the coverage boundary is real and a future cascade that wants to choose a face must route through the manager or repeat the check. Also corrected the opening line, which claimed a symmetry the function deliberately does not have.'
status: addressed
---
