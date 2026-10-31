---
id: RR-W1YVQR
type: review-response
title: goTo skips RlChannel's jumping guard
finding: 'RlChannel.jumpTo sets jumping before it emits jump, so pages of hidden messages loaded for a jump are not announced to screen readers just before reveal shows one (RlChannel.vue:443-454). The plan''s goTo calls reveal and then onJump straight from the page, so jumping stays false and each older page loaded for a message link is announced. Fix: expose jumpTo, or a jumpTo-like method, from RlChannel and call that. This is a small library change, so the claim "no rela-components change" no longer holds.'
severity: minor
resolution: ADDRESSED RlChannel exposes its existing jumpTo and OpenChannel calls it, so link paging keeps the jumping guard; one-line rela-components change listed; case "older pages for a link are not announced".
status: addressed
---
