---
id: RR-AIXFUKC
type: review-response
title: Error hint can point the wrong way
finding: 'The guard error told the operator to list the field in every visible: block, which does not help when a client baseline hides it. The template error had no hint.'
severity: minor
resolution: 'Both messages now say the read is allowed once every role''s visible: block lists it without a when: and no client baseline hides it.'
status: addressed
---
