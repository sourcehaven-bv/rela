---
id: RR-GS8Y15
type: review-response
title: Relation GET missed identity edges on a faced source
finding: relation_read_handler read the edge at the addressed face. An identity-scoped edge is stored at the zero tail. So on a faced source it was never found. A bare id on a faced type also resolved at the zero face and missed (design 8.2).
severity: significant
resolution: 'relationTailOr404 gates an identity edge named by bare id at entity level: the world''s face when readable and otherwise the first readable face from Family. The edge is read at the zero tail for identity relations. TestRelationGet_IdentityEdgeOnFacedSource covers bare and addressed reads and a denied face. PATCH and DELETE already read the tail off the stored edge.'
status: addressed
---
