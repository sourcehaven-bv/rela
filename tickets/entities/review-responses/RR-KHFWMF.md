---
id: RR-KHFWMF
type: review-response
title: ResolveHeadersErr answers an unset world with an empty map
finding: internal/visibility/batch.go ResolveHeadersErr returns empty and nil when the world is unset, so a wiring fault becomes an empty pile (200) instead of an error, contradicting readableItems' contract.
severity: minor
resolution: ResolveHeadersErr returns an error wrapping store.ErrInvalidQuery for an unset world (same sentinel as WriteTarget); both callers always set a world. batch_err_test updated.
status: addressed
---
