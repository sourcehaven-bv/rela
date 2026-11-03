---
id: RR-ANMUZP
type: review-response
title: Creator-derived transition role cached as not held before the first post creates the channel
finding: 'internal/realtime: Post calls mayPost, which fills the shared per-call role cache before s.record(c) stores a cardinality-one channel. A transition role from creator: is then evaluated with no stored channel and cached as not held, so the first post that creates the channel does not move the session. Fix: do not cache creator-derived answers before the channel exists (or re-evaluate after record), and add a test for the first post moving the session with a creator role. Reported by rela-security-reviewer as a non-security note.'
severity: minor
resolution: A creator-role answer is not cached while the call has no stored channel; TestSession_FirstPostByCreatorMoves pins it.
status: addressed
---
