---
id: RR-0I2Q3O2
type: review-response
title: Unquoted bash pattern replacement garbles issue body
finding: bash 5.2 patsub_replacement expands an unquoted & in ${BODY//x/$LIST}
severity: significant
resolution: Replacements quoted
status: addressed
---
