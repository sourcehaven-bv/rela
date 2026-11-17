---
id: RR-YXYX9EQ
type: review-response
title: The enforcer joins the flows.yaml loader package
finding: EnforceUpdate and Performable run on every write with an injected store binder, and flowcfg claims to be pure. Write-path consumers would inherit whatever flowcfg imports, including what TKT-GWOBH2U adds.
severity: significant
resolution: 'The code moved to its own package internal/flowcfg/transitions. Its package doc says it compiles the schema transitions into headless flows and evaluates moves through injected collaborators only. Its arch-lint component transitions carries a GUARD comment: no store, lua, jobs or automation dependency.'
status: addressed
---
