---
id: RR-WGGJ61
type: review-response
title: send_typing gives agents little value at a 6 s TTL
finding: 'An agent cannot refresh while it generates, and an LLM turn usually takes longer than TypingTTL (6 s). One send_typing call therefore shows the indicator for 6 s and then drops it mid-generation. Every in-process MCP client also pays for one more tool in its tool list. Plan change: drop send_typing from this ticket (simplest), or record why 6 s is useful for agents. If it stays, add a guide note that agents should call it right before posting.'
severity: minor
resolution: 'Plan updated: send_typing is dropped from scope with the reason (TTL shorter than a generation); read_channel shows typing_now and actions.typing.'
status: addressed
---
