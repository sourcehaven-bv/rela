---
id: RR-OCZZEEX
type: review-response
title: Later batch messages judged against the state message 1 created
finding: 'After message 1 moves the session, Admit checks in: of later messages against the new state and Decide may move again, clearing state and adding notices; the service pre-check uses the old head.'
severity: significant
resolution: 'Plan: a batch is one post for the session: every message is admitted against the pre-batch state, Decide runs per message until one moves, and later messages are not decided. A batch moves the session at most once. Documented in the guide.'
status: addressed
---
