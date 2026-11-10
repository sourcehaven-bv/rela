---
id: RR-A3XZWQF
type: review-response
title: No nil metamodel guard in elevation validation
finding: validateActionElevation called meta.GetEntityDef without a nil check.
severity: nit
resolution: The type checks are skipped when meta is nil, as elsewhere in validate.go.
status: addressed
---
