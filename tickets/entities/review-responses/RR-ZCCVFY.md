---
id: RR-ZCCVFY
type: review-response
title: notePosted is a funlen dodge with an odd shape
finding: 'internal/realtime/service.go:457 moves ten unrelated lines out of Post into a free function notePosted(l *Local, c *call, m). It exists only to keep Post under the funlen statement cap after the two new in: branches. That is acceptable, but the signature passes s.local as a parameter next to c, which already reaches everything. Make it a method, (l *Local) notePosted(c *call, m channels.Message), or (s *Service) notePosted(c, m), so the call site reads s.notePosted(c, m) and the free function does not suggest it is reusable outside Post. It is churn unrelated to the ticket, so keep it as small as possible.'
severity: nit
resolution: notePosted is now a method, (l *Local) notePosted(c, m).
status: addressed
---
