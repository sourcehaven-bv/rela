---
id: RR-HE7LJP
type: review-response
title: Adding a session to an existing type lights up old channels
finding: 'busy.go:91 queries "" when the initial state is working, and currentSession reads every never-moved channel as in the initial state since CreatedAt. If an operator adds a session with an explicit `status: working` initial state to a type that already has channels, every open old channel turns busy at once, with "Planning · 120d". The load warning does not fire, because the status is declared. Fix: document this in the "A working initial state" paragraph of GUIDE-realtime.md, or warn whenever the initial state of a busy type is working (declared or default).'
severity: minor
resolution: 'Documented in GUIDE-realtime: adding a session whose initial state is working to a type with existing channels makes every never-moved channel busy from its creation time, with advice to start in a waiting state; no code change because the channel has no record of when the session was added.'
status: addressed
---
