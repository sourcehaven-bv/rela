---
id: RR-UD2G
type: review-response
title: propertyName semantics align across form and view (no issue)
finding: |
  Verified: form-side field.property is the metamodel property name (FormFieldOrRelation.property); view-side both field.property and field.propType are documented as "Raw property name". PropertyDisplay's prop.propType ?? prop.name is correct. This is fine.
severity: nit
status: addressed
resolution: |
  No code change needed -- reviewer confirmed the semantic alignment between form and view-side propertyName during the review itself. Closed at intake.
---
