---
id: RR-0FPXW
type: review-response
title: useConfirmHost returns mutable reactive state instead of readonly
finding: Consumers should not mutate state. Returning readonly(state) catches future drift cheaply.
severity: minor
resolution: useConfirmHost now returns readonly(state) using Vue's readonly helper. Type signature changed to DeepReadonly<ConfirmState>. App.vue and test harnesses still work because they only read state.
status: addressed
---
