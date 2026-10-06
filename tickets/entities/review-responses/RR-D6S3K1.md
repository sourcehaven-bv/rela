---
id: RR-D6S3K1
type: review-response
title: Property-name lookup can borrow another type's labels
finding: scanPropertyDefs falls through to other types when the given type's def yields nothing. With the real property name, a string field on one type could show a same-named enum's label or colour from another type.
severity: significant
resolution: scanPropertyDefs now stops at the given type's def when that type declares the property. Unit test added in schema.test.ts.
status: addressed
---
