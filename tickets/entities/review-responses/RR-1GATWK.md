---
id: RR-1GATWK
type: review-response
title: Confirm asked before the permission and entity gates
finding: RunAction returned confirmation_required before run checked permission, entity and available_on, so a caller could be asked to confirm a run that was never allowed.
severity: significant
resolution: actionOps.run takes a beforeScript hook that runs after every gate; MCPActions passes the confirm check there. TestMCPActions_ConfirmAfterGates pins permission_required, entity_not_found and action_not_available without confirm.
status: addressed
---
