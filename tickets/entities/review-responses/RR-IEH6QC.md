---
id: RR-IEH6QC
type: review-response
title: Conflicted item hidden by 304
finding: An item ending in ErrConflictExhausted was not marked seen, but the validators were saved, so later polls got 304 and never retried it.
severity: minor
resolution: A run with a conflicted item saves no validators, so the next poll refetches. New Conflicted counter; test asserts no If-None-Match.
status: addressed
---
