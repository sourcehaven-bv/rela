---
id: RR-0QLLXJG
type: review-response
title: Postgres VALUES join needs casts
finding: Joining bigint seq against VALUES parameters makes postgres infer text.
severity: minor
resolution: 'Two queries: channel ids by text key columns, then messages by (channel_id, seq) with CAST(? AS BIGINT); conformance runs on postgres.'
status: addressed
---
