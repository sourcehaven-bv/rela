---
id: RR-0TBFZZ
type: review-response
title: Docs claim a transition type is never selected by default; the code selects one when nothing else is offered
finding: 'frontend/packages/rela-components/README.md:1002-1006 says the composer "never selects one by default". GUIDE-realtime.md (In the web app) says "It still selects a type that moves nothing by default". RlChannelComposer.vue:144-147 (defaultType) falls back to the first transition type when every offered type is a transition, and the OnlyTransitionTypes and OneTransitionType stories pin that fallback. Fix: state the fallback in both docs ("...when one is offered; otherwise the first, and the note says the session will move").'
severity: minor
resolution: README and GUIDE-realtime say a transition type is selected by default only when every offered type is a transition; docs regenerated.
status: addressed
---
