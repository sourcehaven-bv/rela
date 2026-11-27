---
id: RR-CLGY59Q
type: review-response
title: Trigger set wrote the source without update authorization
finding: A relation grant or create permission let a trigger set write fields the caller could not update\; identity edges wrote every readable face.
severity: significant
resolution: authorizeTriggerWrite runs the read gate and the update authorization on each written row; a refusal fails the relation write.
status: addressed
---
