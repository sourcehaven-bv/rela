---
id: RR-O6MDNF
type: review-response
title: 'Stub resolver instead of a declarative fields: rule'
finding: The test uses newVerdicts instead of an acl.yaml rule.
severity: nit
reason: 'Won''t fix: the real resolver has no file-specific branch and the verdict path is shared with PATCH, whose declarative tests cover it.'
status: wont-fix
---
