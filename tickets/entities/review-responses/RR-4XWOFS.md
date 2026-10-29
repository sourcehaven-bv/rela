---
id: RR-4XWOFS
type: review-response
title: choiceViews cannot share the decisions memo key
finding: 'Security/Cost section (plan line 592) says choiceViews is "memoized on the same key as decisions". That key (useSessionDecisions.ts:116-126) covers only head and actionsGeneration; choiceViews also reads the held messages (answered check, removal, question props, edits). With that key, a held answer arriving by a live frame does not hide the buttons in a channel without a session (the head key may not change), and an edited or removed question keeps stale buttons. Fix: key the choices memo on the head key plus the messages array identity (shallowRef, replaced on each merge) or on the seq/rev of held messages of choice-bearing and answer types; add a channelView/useSessionDecisions test where only `messages` changes and the buttons hide.'
severity: significant
resolution: 'The choices memo has its own key: the decisions key plus hasNewer plus the identity of the messages shallowRef, which is replaced on every merge. A test checks that the buttons hide when only messages changes.'
status: addressed
---
