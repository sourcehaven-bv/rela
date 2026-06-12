---
id: RR-KBGY
type: review-response
title: AC3 byte-equal regression test is fragile against non-deterministic fields
finding: 'AC3 says ''encode list+GET response for fixture entities; diff against golden JSON from today''s behaviour.'' That assumes deterministic property iteration, deterministic relation iteration, and no timestamps in the wire body — fine today, fragile against any future maintainer adding a `served_at` or `_request_id` field. Fix: make AC3 explicitly ''structural-equal'' rather than byte-equal: compare via JSON-canonical with known-volatile keys dropped (e.g. a documented allowlist). Same lesson applies to AC5 (404-vs-deny byte-equal): audit writeV1Error for per-request volatility (X-Request-ID, timestamps) and pin the same canonicalization. Otherwise AC5 false-fails on unrelated changes and is silently disabled.'
severity: significant
resolution: 'Addressed in rescoped TKT-VQGN AC1 and AC6: ''structural-equal, JSON-canonical'' compare (not byte-equal) with documented volatile-field allowlist. Same wording propagated to TKT-VMD8 AC8.'
status: addressed
---
