---
id: RR-5LJKGD
type: review-response
title: Boolean flag parameter on fallback
finding: 'live.go:923 fallback(pairs, topic bool): the call sites pass only != nil, and the bool hides what it means. Fix: pass only (*roundFilter) and test only != nil inside, or use a named type such as roundKind with fullRound and topicRound constants.'
severity: nit
resolution: 'Fixed: fallback takes a roundKind (fullRound, topicRound) built by kindOf(only).'
status: addressed
---
