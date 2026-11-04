---
id: RR-2N6HFSL
type: review-response
title: Go to decision can do nothing
finding: OpenChannel.vue goToDecision ignored a false result of focusDecisions.
severity: minor
resolution: 'When focusDecisions returns false it calls reveal(seq). Test: the busy button of a decision in flight is aria-disabled, not disabled, so it still takes the focus; pinned in ChannelsPanel.test.ts.'
status: addressed
---
