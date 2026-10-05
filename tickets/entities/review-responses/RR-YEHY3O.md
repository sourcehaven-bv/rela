---
id: RR-YEHY3O
type: review-response
title: Zero-tail content edge from a faced source was checked at the zero face only
finding: relationWriteSubject set FamilyFaces only for identity-scoped relations. A content edge written at the zero tail on a faced source was checked at the default face, which a bare-type or * grant covers, so it needed no face grant while an identity edge at the same address needed every face.
severity: significant
resolution: 'Any zero-tailed edge from a faced source is now authorized on every stored face. Tests: TestCreateRelation_ZeroTailOnFacedSourceNeedsEveryFace and TestDelete_CascadeIdentityEdgeFromFacedSourceNeedsEveryFace (manager level, memstore and fsstore).'
status: addressed
---
