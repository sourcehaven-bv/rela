---
id: RR-5HPTYX
type: review-response
title: session_moved is handled in two places
finding: 'useSessionDecisions.ts:122-126 catches session_moved and session_wrong_state, toasts decisionStale and refetches. channelErrors.ts:167-171 adds a classifyChannelError arm for session_moved that does the same (REFETCH + decisionStale). Only decision buttons send expect_generation (the comment says so), and they never reach classify for that type, so the classify arm is dead in practice and its message can drift from the composable''s. Pick one: let `refused` override only session_wrong_state and pass session_moved through d.handle, or keep the composable arm and drop the comment''s claim. The CHANNEL_ERROR_RULES entry is still needed for channelErrors.test.ts.'
severity: minor
resolution: refused() now handles only session_wrong_state itself; session_moved goes through the panel's handle(), whose classify arm gives the decisionStale message and the refetch.
status: addressed
---
