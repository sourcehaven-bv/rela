---
id: RR-AWA8CI
type: review-response
title: Picking a ref value does not count as typing
finding: 'RlChannelComposer.vue:222 setRef never emits typing, while setValue (:215) does. A user filling a ref-only log message (e.g. selecting an entity, then submitting) never shows as typing. Fix: emit typing from setRef when r is set, under the same !editing && persist === log guard; or state in the component doc that only field edits count.'
severity: nit
reason: 'Not a defect: picking a ref goes through setValue, which already emits typing; emitting from setRef too would double it; a story check pins the behaviour.'
status: wont-fix
---
