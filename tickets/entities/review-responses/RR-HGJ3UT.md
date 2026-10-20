---
id: RR-HGJ3UT
type: review-response
title: Composed and caption strings bypass the library translation seam
finding: 'The library README (Composed strings and translation) says captions arrive as props and composed strings go through useMessages. The channel components add five useMessages entries but hardcode the rest. Composed with a value: RlChannelComposer.vue:181 `At most ${max} characters.` (the exact case the README names). Fixed captions with no prop: RlChannelComposer.vue:179 ''Required.'', 234 ''Edit message''/''New message'', 244 ''Message type'', 254 ''Cancel reply'', 282 ''to save/send'', 286-296 Cancel/Save/Send/Saving…/Sending…. RlChannel.vue:156 Closed/Archived, 167 ''Working…'', 175 ''Messages'', 178 ''Load older messages'', 181 ''No messages yet.'', 204 ''Jump to latest'', 223-224 closed/archived sentences. RlChannelMessage.vue:124 Yes/No, 147 ''(edited)'', 244-277 Release/Reply/Edit/Delete/Remove… and their pending labels. RlChannelField.vue:53 ''—'', 136/151 Choose…/Change…/Clear. RlEntityChannels.vue:33-36, 44, 61, 81, 85. RlChannelAuthor.vue:217-218 ''system''/''agent''. RlRefChip.vue:308-310. Fix: move the value-composed ones (max, removed/archived sentences if they ever take a name) into Messages now. Give captions either props with English defaults, like RlCommentComposerBox''s submitLabel and placeholder, or Messages entries for the many fixed ones. Otherwise the first localised consumer has to fork these components.'
severity: significant
resolution: 'All channel captions now go through useMessages: a keyed channelText table for fixed strings, plus channelTooLong and channelNewMessages for composed ones.'
status: addressed
---
