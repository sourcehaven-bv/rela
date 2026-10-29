---
id: RR-YJ9NGF
type: review-response
title: Other label is cached across a locale switch
finding: 'channelView.ts:518 calls t(''channels.choiceOther'') inside choiceViews, but the choicesMemo key (useSessionDecisions.ts:167-186) does not include the locale. After a language switch, the Other button keeps the old language until the messages or head change. Fix: add the current locale to the memo key, or let RlChannelDecisions resolve the Other label from a flag instead of a baked string.'
severity: nit
resolution: Added the current locale (i18n.global.locale.value) to the choicesMemo key. Added a test that switches to nl and expects 'Ander antwoord…'; it fails without the fix.
status: addressed
---
