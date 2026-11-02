---
id: RR-VZBP15
type: review-response
title: Order semantics of in differ by source
finding: 'A written in: is served in written order. A derived in: is served in session.states order. The table documents this, but the OpenAPI, TS and guide text do not. Say there that the order carries no meaning, or normalize a written list to session.states order. Either choice keeps clients from depending on it.'
severity: nit
resolution: "addressed\tThe v1 doc, OpenAPI description and guide state that the order of in carries no meaning; normalizing was rejected because the served list would then differ from the file."
status: addressed
---
