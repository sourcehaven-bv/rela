---
id: RR-UZHHBJ
type: review-response
title: One-click post skips typing.settle()/reset()
finding: 'onPost awaits typing.settle() before ch.post and resets typing afterwards (OpenChannel.vue:279-286), because a log post ends the typing entry on the server and a typing call still in flight would bring it back. A user with a half-written draft who clicks a decision hits exactly this: a ghost ''Owner is typing...'' line stays until the server expires it. Do the same settle/reset in onDecide, or factor a shared postTyped helper.'
severity: minor
resolution: A shared postTyped helper does typing.settle() before and typing.reset() after both the composer post and the one-click post.
status: addressed
---
