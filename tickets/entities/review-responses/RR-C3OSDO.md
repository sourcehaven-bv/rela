---
id: RR-C3OSDO
type: review-response
title: 'sweepNow builds a sweep with nil cancel and done, one refactor away from a permanent block'
finding: 'internal/store/sqlitestore/sweep.go: the test-only synchronous tick constructed &sweep{...} without cancel or done. tick touches neither so it worked, but stop() on that value would call a nil cancel and then receive from a nil channel, blocking its caller forever.'
severity: minor
resolution: 'sweepNow now builds a fully formed sweep: a closed done channel and a real cancel derived from the caller''s context. Two lines to remove a latent deadlock that only shows up after someone else''s refactor.'
status: addressed
---
