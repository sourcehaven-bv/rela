---
id: RR-INUNSH
type: review-response
title: Gaps in test coverage for the plan's scroll and focus edge cases
finding: 'Not covered: (a) a config reload that changes display keeps the view (AC 7; the watch on props.presentation in RlChannel.vue has no test); (b) focus inside an expanded run survives an append (AC 5; only focus on the fold button is tested); (c) after the tail fold is expanded at the bottom, a live append does not scroll to the end (plan edge case; the ''Quiet types'' story never appends after the expand); (d) revealing a hidden member of a collapsed run splits the fold; (e) a reload that removes every quiet type while the toggle is on removes the toggle. Fix: add (a), (c) and (e) as story play steps (they need layout) and (b) and (d) as ChannelsPanel or channelFold cases.'
severity: minor
resolution: 'Fixed. (a) and (e) in new story "Quiet types, config reload" (a reload without display keeps the first visible fold line''s place, removes the toggle, also while it is on). (c) in "Quiet types": after the tail expand, an append keeps the fold''s offset and does not scroll to the end. (b) panel test "focus inside an expanded run survives an append". (d) channelFold case "revealed hidden member splits the fold".'
status: addressed
---
