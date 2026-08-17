---
id: RR-TXMRYN
type: review-response
title: 'A write fired after the component unmounted'
finding: 'propose() is a free-floating promise and useConfirm only resolves pending promises on APP-shell unmount, not component unmount. Answering the clear dialog after navigating away resumed the awaited continuation on a dead component and scheduled a real PATCH against a form the user had left. Reproduced: 1 update call after unmount.'
severity: critical
resolution: 'onBeforeUnmount now bumps formGeneration, which the existing generation fence already checks - the proposal resolves as superseded instead of applying. Mutation-checked: removing the bump reproduces the write.'
status: addressed
---
