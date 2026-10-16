---
id: RR-CVD9V5
type: review-response
title: Guard errors name hidden owned entity ids
finding: channelguard.go list/detach errors include the owned entity id; MCP and Lua return err.Error() to the caller, bypassing the ownedDeleteError rule that names only the owner.
severity: minor
resolution: Guard errors for owned entities go through ownedDeleteError or withoutDetail and never name the owned id; Scope errors name no ids.
status: addressed
---
