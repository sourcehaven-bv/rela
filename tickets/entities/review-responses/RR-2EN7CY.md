---
id: RR-2EN7CY
type: review-response
title: A stale choice in a channel without a session says 'The session moved on'
finding: 'useSessionDecisions.ts:356 (and the stale() call in openForm) shows channels.decisionStale (''The session moved on. Choose again.'') when a choice key is no longer offered. Without a session this happens when someone else answered first, and the text names a session that does not exist. useSessionDecisions.test.ts ''a key that is no longer offered'' pins that wrong text with choiceSetup(false). Fix: add channels.choiceStale (''This question is no longer open.'' / ''Deze vraag is niet meer open.'') and use it for choices when the channel type has no session.'
severity: minor
resolution: Added channels.choiceStale ('This question is no longer open.' / 'Deze vraag is niet meer open.'). onChoose uses it for an unknown key, and openForm uses it (through a new onStale parameter) when Other's type is no longer offered, but only if the channel type has no session. The 'no longer offered' test now covers both cases, and a new test covers Other without a session. Documented in the guide.
status: addressed
---
