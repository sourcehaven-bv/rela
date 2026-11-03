---
id: RR-Q7EPIN
type: review-response
title: The existing MCP sentence still calls every button one-click
finding: 'Approach 7 appends a sentence. Both descriptions still say "A row of session.transitions with button is a one-click preset that posts its props" (internal/mcp/tools_channels.go:283-284, 336), and the appended text contradicts that. Agents read these descriptions literally. Rewrite the existing sentence rather than appending, for example "...with button is a preset; without compose it is one-click and posts its props". Also update the godoc that says one-click: TransitionButton (internal/realtimecfg/buttons.go:20-28), SessionButtonView (internal/realtime/session.go:119-121), apiwire ChannelSessionButton (internal/apiwire/v1/channels.go:159-162), and the presetViews comment "A preset is one-click" (frontend/src/components/entity/channels/presetViews.ts:120). None of these files'' doc changes are listed.'
severity: minor
resolution: ADDRESSED The existing MCP sentence is rewritten, not appended to. Godoc of TransitionButton, SessionButtonView, TransitionView.Button and apiwire ChannelSessionButton, and the presetViews comment, are listed (AC 6, Approach 6, 7, 10).
status: addressed
---
