---
id: RR-GOIEXC
type: review-response
title: Creator-role check duplicated instead of reusing call.can
finding: 'internal/realtime/waiting.go:116-117 copies the SourceCreator branch of call.can (authz.go:274-279), including the ceiling rule. If can''s creator or ceiling rule changes, waiting_on_me and the post gate drift apart without any test noticing. Fix: answer through c.can with c.head temporarily set to h, or extract a creatorHolds(h) helper that both use.'
severity: nit
resolution: call.can and waitingOf share roleAdmits for the creator and ceiling logic
status: addressed
---
