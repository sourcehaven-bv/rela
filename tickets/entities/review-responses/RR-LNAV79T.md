---
id: RR-LNAV79T
type: review-response
title: Headless flows hidden when flows.yaml or acl.yaml is broken
finding: loadProjectFlows returns early on an acl.yaml or flows.yaml error, which would also hide the schema transitions.
severity: minor
resolution: The CLI compiles the headless flows on their own, and a flows.yaml error no longer hides them.
status: addressed
---
