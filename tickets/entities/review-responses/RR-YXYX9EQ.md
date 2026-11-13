---
id: RR-YXYX9EQ
type: review-response
title: The enforcer joins the flows.yaml loader package
finding: EnforceUpdate and Performable run on every write with an injected store binder, and flowcfg claims to be pure. Write-path consumers would inherit whatever flowcfg imports, including what TKT-GWOBH2U adds.
severity: significant
resolution: 'The flowcfg package doc now says it compiles both kinds of flows and evaluates headless moves through injected collaborators only. The arch-lint entry for flowcfg gets a GUARD comment: no store, lua, jobs or automation dependency.'
status: addressed
---
