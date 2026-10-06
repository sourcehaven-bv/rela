---
id: RR-4W8DHZ
type: review-response
title: Face hint computed twice per row
finding: offWorldFace ran entityRef and faceLabel twice per rendered row; the hint is also a second face-labelling UI beside WorldBadge.
severity: nit
resolution: 'The label is computed once per load into the hint map; the template does a map lookup. A WorldBadge variant was not added: WorldBadge describes how the ambient world served a row; which is the wrong statement for an off-world row.'
status: addressed
---
