---
id: RR-J62YPKF
type: review-response
title: Idempotency key must be the unique field in River args
finding: River ByArgs uniqueness hashes the whole args unless a field is tagged; with the payload included two jobs with one key and different payloads would both be queued.
severity: minor
resolution: 'Addressed: riverArgs.Key carries idempotencyID and is the only river:"unique" field.'
status: addressed
---
