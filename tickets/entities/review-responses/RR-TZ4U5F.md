---
id: RR-TZ4U5F
type: review-response
title: Document loadEntry widens the unguarded anchored-document face read
finding: '[security] loadEntry makes the document service read the addressed face. resolveAnchoredDocument had no face gate (BUG-6DBV6N), so a reader holding only one face could render another by spelling its address.'
severity: significant
resolution: 'Landed the BUG-6DBV6N gate in resolveAnchoredDocument: faceReadable runs before any render. TestFaceGrant_AnchoredDocumentIsFaceGated pins both document routes and that the renderer never runs.'
status: addressed
---
