---
id: RR-ZREWRW
type: review-response
title: Lease test hygiene
finding: TestLeaseFailed_Texts and TestRenewLease_StatusAndTTL lack t.Run; no ErrInvalid or wrapped-sentinel rows; no ID@face or malformed parity rows
severity: minor
resolution: 'Fixed in the review-fix commit (Lease MCP tools: review fixes) with tests where behaviour changed.'
status: addressed
---
