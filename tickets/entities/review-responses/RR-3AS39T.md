---
id: RR-3AS39T
type: review-response
title: 'Q2: yes, count defaulted working, but warn about initial-working types'
finding: 'Agree that the board should follow read_session''s status. The real hazard is a type whose initial state defaults to working. Through "" every open channel of that type that never moved shows busy, for example a discussion channel created by a plain post. Because busy: is opt-in per board this stays contained. The plan should add a load-time warning when a busy: type''s initial state reads working without an explicit status:, and document it.'
severity: minor
resolution: AC 8 and step 6 add a load warning when a busy type's initial state reads working by default, using a new SessionState.StatusDeclared; the docs explain it.
status: addressed
---
