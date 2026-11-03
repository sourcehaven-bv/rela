---
id: RR-4W5QKE
type: review-response
title: metamodel.Validator name overlaps validator.Validator
finding: Two types named Validator in different packages read ambiguously in entitymanager.
severity: nit
reason: The package qualifier disambiguates at every call site; TKT-ZTGLU4 renames the package to schema, which is the point to revisit the name.
status: wont-fix
---
