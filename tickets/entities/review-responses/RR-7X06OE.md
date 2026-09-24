---
id: RR-7X06OE
type: review-response
title: Raw traversals ignore the subject's face
finding: Match without World/FaceIn reads the default face's edges; a named-face subject on automation; ACL; state machine or validation is answered from another face's edges.
severity: significant
resolution: 'Every raw surface refuses related() for a named-face subject (error: automation does not fire; grant denied; transition blocked; validation load error). Automation done with TestProcess_TraversalOnNamedFaceDoesNotFire.'
status: addressed
---
