---
id: RR-0VUEF6
type: review-response
title: stylesForProperty fallback comment misstated the fallback's role
finding: Reviewer read the property-name fallback as effectively dead for server-authored data and asked for the comment to say it's defensive-only.
severity: minor
resolution: 'Investigation showed the opposite: the fallback is load-bearing — EntityDetail view sections pass the property''s TYPE name as :property (server PropType), which matches no property def and resolves only via the direct-key fallback. Comment rewritten to document that contract, and a schema.test.ts case pins it.'
status: addressed
---
