---
id: RR-BMHT3PN
type: review-response
title: JS trim and Go TrimSpace differ on U+FEFF and U+0085
finding: 'filters.ts: the outer-whitespace check uses trim(), which differs from Go strings.TrimSpace on U+FEFF and U+0085.'
severity: nit
reason: No real schema name starts or ends with either character; the server judges names on its own. The difference is noted in the doc comment.
status: wont-fix
---
