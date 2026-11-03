---
id: RR-NF4V84
type: review-response
title: A removed answer keeps a question answered for good
finding: 'frontend/src/components/entity/channels/channelView.ts:501 adds every held message of the answer type, removed ones included, to answered. In a channel without a session, a moderator who removes a wrong or abusive answer leaves the question with no buttons at all, and the person must type the answer by hand. The notes defend this as a literal reading of the plan, but ''held'' was meant as ''in the held page'', not ''including tombstones''. Fix: skip m.removed when building answered, and add a choiceViews case ''a removed answer brings the buttons back''. Or, if you keep the current behaviour, state it in docs/realtime.md under Choices.'
severity: minor
resolution: choiceViews now skips removed answers when it builds the answered set, so removing a wrong answer brings the buttons back. Added the choiceViews case 'a removed answer brings the buttons back' and documented it in the guide's list of conditions for the buttons.
status: addressed
---
