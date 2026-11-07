---
id: RR-RNBQWH5
type: review-response
title: Face toggle drops worldForFace semantics
finding: Plan step 6 reuses goToFace (EntityDetail.vue:1462), which pushes the literal @face address and keeps ?world=. Related sections then resolve in the old world, and the sidebar control can show one world while the record toggle shows another face.
severity: significant
resolution: 'Addressed in plan: a toggle segment links to the face address with ?world= set to worldForFace(type, face) when a world heads that face (through worldQuery). With no such world the literal address keeps the current world, as FaceMenu does today.'
status: addressed
---
