---
id: RR-KV766B
type: review-response
title: One-click amplifies the stale-approval race on re-entered states (allow in approving)
finding: 'No client lock can stop this, and in: or S5OJ7S do not help: approving admits allow every time. Sequence: tab or phone A clicks Allow (gen 5 to working); the runner immediately posts the next permission (gen 7, approving again); device B''s head still shows the first request and its Allow lands in the new approving state and approves a tool call the owner never saw. The message-row placement makes it worse, because the button sits under permission #N while the post carries no reference to #N (Open question 3: no reply_to). Options: (1) server precondition (`expect_generation` on POST, refused as 409 session_wrong_state); this contradicts ''SPA only'' but is the real fix; (2) at least post `reply_to: decisionSeq` when the cause is known (PoC types are log, so reply is allowed). That binds the decision to the request it answers, in the log and for the agent, which can then reject a mismatch. Record the chosen trade-off in Risks; today it is not mentioned.'
severity: significant
resolution: 'Added a server precondition: optional expect_generation on POST, MCP post_message and rela channel post, checked in SessionStep.Admit under the store lock and refused with 409 session_moved, covered by realtime, dataentry, MCP, CLI, OpenAPI and a channelstest conformance case; buttons also send reply_to the deciding message when it is top-level and the type may reply.'
status: addressed
---
