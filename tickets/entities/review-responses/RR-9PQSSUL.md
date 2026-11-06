---
id: RR-9PQSSUL
type: review-response
title: Undeclared properties made some conflicts unresolvable
finding: CheckFieldWrite refuses a name that is neither declared nor known to the resolver. A resolve writes every disagreed key so a legacy frontmatter key that differs between branches blocked every principal.
severity: significant
resolution: validateConflictFieldWrite leaves out a key that the schema does not declare and no field rule names. The caller has seen the name in the detail and no rule governs it. Covered by TestV1ConflictResolve_FieldWriteCases.
status: addressed
---
