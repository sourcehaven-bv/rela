---
id: RR-GSS3QJ
type: review-response
title: Postgres caps and clock races
finding: 'Security design review: count-then-insert can exceed caps; node clock skew can steal a live lease.'
severity: minor
resolution: 'Plan: caps under pg_advisory_xact_lock(holder), expiry compared with db now(), expired rows excluded before counting, limits after access checks, generic store-full error.'
status: addressed
---
