---
id: RR-51GFZWP
type: review-response
title: "Bulk bar plural is untrimmed and changed without intent"
finding: "With label_plural on the wire the bulk bar uses it untrimmed; a blank label reaches the bar as blank text."
severity: significant
resolution: "The server trims label_plural (TestToV1EntityType_LabelPlural covers a blank one) and the bulk bar reads it through schemaStore.pluralLabel."
status: addressed
---
