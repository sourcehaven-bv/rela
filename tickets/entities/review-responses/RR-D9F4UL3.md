---
id: RR-D9F4UL3
type: review-response
title: EntityDetail title edit chose its save target at commit time
finding: saveTitle took editAutoSave() on commit. EntityTitle kept its draft across a row switch, so committing after a keyboard Back wrote A's draft as B's title.
severity: significant
resolution: 'EntityTitle emits edit; EntityDetail pins the instance and the title property at edit start and applies the optimistic heading only when that row is on screen. EntityTitle is keyed by servedRef, so a switch closes the edit. Test: ''closes an open title edit when the page switches row''.'
status: addressed
---
