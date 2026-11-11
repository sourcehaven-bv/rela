---
id: RR-VW4MTWN
type: review-response
title: 'Attribution string reads role=outside_edits:'
finding: The audit attribution for a ceiling denial reuses the role= format, which reads oddly.
severity: nit
reason: It is the existing attribution format of field verdicts; changing it touches every consumer of the audit field.
status: wont-fix
---
