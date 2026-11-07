---
id: IDEA-DWYDESU
type: idea
title: Agent work as a durable flow run
description: 'Model an agent''s ticket work as a flow run: each model call and tool call is a step with saved progress, a lease per step, and MCP step tools generated from the step''s writes so their schema matches the ACL.'
category: ai
inspiration: waymark.sh agent tutorial (each model and tool call is a durable step)
effort: medium
value: valuable
notes: Add a step cap or deadline; Waymark has no turn or budget limit. Depends on flows M1 runtime and M2 step tools.
status: captured
---
