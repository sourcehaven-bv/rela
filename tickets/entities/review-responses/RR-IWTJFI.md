---
id: RR-IWTJFI
type: review-response
title: The 'older messages are hidden' message also plays during a reply-quote jump
finding: 'When onJump (useChannelPaging.ts:48-52) loads an older page that holds only hidden messages, announce (RlChannel.vue:309-312) says ''N older messages are hidden. Use Show all messages to see them.''. Right after that, onHeld runs reveal, which shows and focuses one of those messages. The screen reader hears a hint that contradicts what happens next. Fix: give reveal() or RlChannel a ''jumping'' flag that skips the older-hidden message, or have the jump use its own load path, or clear the announcement in reveal() when it reveals a seq from that page.'
severity: minor
resolution: Fixed. RlChannel sets a jumping flag when it emits jump; announce skips the older-hidden message while it is set. reveal() and a click on Load older clear it. "reply quote reveals a hidden target on an older page" now serves an all-hidden older page and asserts no announcement; it fails without the flag.
status: addressed
---
