---
id: RR-D6W86R
type: review-response
title: No real DST-transition instant is tested
finding: 'The ''DST is handled'' claim in localInputToUtcISO''s docstring is never exercised by a spring-forward/fall-back instant; the emit test uses a plain July date (fixed offset). TZDate normalizes a nonexistent gap local time (spring-forward 02:30->03:30) and picks first occurrence on fall-back, which is reasonable but asymmetric on round-trip. Fix: add a test pinning a DST-transition instant, and note in the docstring that gap/overlap local times are normalized (not a true round-trip).'
severity: minor
resolution: 'Fixed. Added a widget test pinning a US spring-forward instant: mount America/New_York, set 2026-03-08T03:30 (just past the 02:00 gap, UTC-4), assert emit = 2026-03-08T07:30:00Z. Also expanded the localInputToUtcISO docstring to note that gap/overlap local times are NORMALIZED by TZDate (not a true round-trip), reachable only via hand-typed boundary times.'
status: addressed
---
