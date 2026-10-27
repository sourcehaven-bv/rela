---
id: RR-VGGM97
type: review-response
title: An agent's post clears the human's unread, because the tool is ignored
finding: 'Decision 1 makes the browser and a remote MCP agent of one user one reader. An own post moves the marker past every earlier message. An agent that works the inbox and posts as the user therefore marks the human''s channels read, including messages from colleagues the human never saw. This follows from decisions 1 and 4, but the plan does not say so. Fix: state it in docs/realtime.md (identity table) and in the mcp-server.md agent loop, and recommend a separate job identity (--run-as) for agents that should not touch the human''s read state.'
severity: minor
resolution: Identity table, decision 1, docs/realtime.md and the mcp-server.md agent loop state that an agent posting as the user moves the user's marker and recommend --run-as.
status: addressed
---
