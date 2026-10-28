---
id: RR-O3XBS3
type: review-response
title: Widget max prop means attachment cap
finding: Every widget receives max=propertyDef.max documented as the attachment cap. Reusing that prop in NumberWidget would blur the two meanings.
severity: nit
resolution: NumberWidget reads propertyDef.min and propertyDef.max explicitly; the wire type documents max per property type.
status: addressed
---
