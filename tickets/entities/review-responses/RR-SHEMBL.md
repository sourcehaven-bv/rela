---
id: RR-SHEMBL
type: review-response
title: EntityTemplatePath left unguarded next to the guarded variant path
finding: The rationale for guarding EntityTemplateVariantPath applies verbatim to EntityTemplatePath (3 raw-FS callers) and RelationTemplatePath.
severity: significant
resolution: Both now validate and return (string; error). Callers in renametype / cli rename / templating updated.
status: addressed
---
