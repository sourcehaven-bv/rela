---
id: RR-ET3DIC
type: review-response
title: post_message description not addressed
finding: internal/mcp/tools_channels.go repeats "A message type may be limited to some session states ... session_wrong_state" in the post_message description. The plan extends only the read_channel sentence. Decide whether post_message points to read_channel's message_types[].in, and write down the decision. An agent that hits session_wrong_state reads post_message first.
severity: minor
resolution: "addressed\tStep 7 extends the post_message description to point to read_channel message_types[].in, since an agent refused with session_wrong_state reads post_message first."
status: addressed
---
