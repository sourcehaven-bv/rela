---
id: RR-DE1HOQ8
type: review-response
title: external_ref key shape unspecified
finding: A path yields a scalar; a ref is (system, id).
severity: significant
resolution: 'The system comes from the schema property; the import writes {id: value} and compares ids. WriteExternalRefs is set only on writes that name an external_ref property.'
status: addressed
---
