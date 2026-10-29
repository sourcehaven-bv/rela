---
id: RR-QN0Y8M
type: review-response
title: Duplicate and unneeded strings
finding: channelDecisionPending() 'Sending'/'Versturen' duplicates the existing ChannelTextKey 'sending' ('Sending...', useMessages.ts:84/399); reuse it. decisionDiscardEditTitle and decisionDiscardEditBody are both given the text 'Discard your edit?'; write both texts in en and nl, or use one key. Consider whether channelDecisionHint can share wording with channelSessionHint/channelSessionOptionHint, so the composer and the buttons describe a move the same way.
severity: nit
resolution: The pending label reuses the existing sending text, the discard-edit dialog has distinct title, body and confirm texts in en and nl, and the button hint wording matches channelSessionOptionHint and channelSessionHint.
status: addressed
---
