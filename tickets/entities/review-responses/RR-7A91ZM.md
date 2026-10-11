---
id: RR-7A91ZM
type: review-response
title: Test asserts error text, not type
finding: strings.Contains(err, invalid relation) cannot tell unknown type from mismatch.
severity: minor
resolution: errors.As on RelationNotFoundError and InvalidRelationError.
status: addressed
---
