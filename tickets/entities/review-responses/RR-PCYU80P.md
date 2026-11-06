---
id: RR-PCYU80P
type: review-response
title: Refused post-create automation write looked like a failed create
finding: An on-create automation whose values break an enforced rule failed the CAS write after the row was stored, and CreateEntity returned a plain 422. A retry would create a duplicate.
severity: significant
resolution: createFollowUps wraps the error in KeptEntityError, whose message says the entity was created. TestWriteRule_PostCreateAutomation pins it.
status: addressed
---
