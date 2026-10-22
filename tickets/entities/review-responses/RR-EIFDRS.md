---
id: RR-EIFDRS
type: review-response
title: Cap refusal charges a typing token, against the documented contract
finding: 'internal/realtime/typing.go:58-63 takes the limiter token (and an IdentityRate aggregate token) before typingStore.touch can refuse with the per-channel or process cap 429, yet the godoc at typing.go:31 says ''A refused call charges no token and keeps no entry''. A caller refused by the cap is also charged, so its retry after RetryAfter can hit the per-identity bucket instead. Fix: check room first (add a typingStore.admits(key, identity, now) under the store lock, or refund on cap refusal), or reword the godoc and docs/realtime.md to say a cap refusal is charged. While there, document that typing also draws on realtimelimits.IdentityRate (300/m shared with posts); the docs only say signal_rate does not apply.'
severity: minor
resolution: Typing checks cap room before charging a rate token; TestTyping_Caps asserts a refusal charges nothing; godoc and the guide say typing shares the per-identity budget.
status: addressed
---
