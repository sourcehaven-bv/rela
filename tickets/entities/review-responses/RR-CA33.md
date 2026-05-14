---
id: RR-CA33
type: review-response
title: 'Architect #2 + cranky #7: double validation on Update; partition once'
finding: 'Manager.UpdateEntity called Meta.ValidateEntity twice on every call: once at the top to partition for hard/abort, again post-automation to capture soft warnings. Wasted full re-validation when automation didn''t run.'
severity: significant
resolution: Gated the post-automation re-validation on len(autoResult.PropertiesSet) > 0. When properties didn't change, reuse the pre-write soft partition (now stored on result.Warnings from the initial partition). Only one ValidateEntity call in the common path.
status: addressed
---
