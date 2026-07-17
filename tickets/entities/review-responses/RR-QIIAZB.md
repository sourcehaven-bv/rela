---
id: RR-QIIAZB
type: review-response
title: Fixtures fallthrough, display format, and DST/offset conversion tests
finding: '(1) testutil/fixtures.go: unknown types fall through to RandomString(), so without a datetime arm, conformance/fuzz tests would feed invalid datetimes and mask bugs - the RandomDatetime arm is required, not optional. (2) Display format for formatDatetime must be pinned deterministically: recommend Intl.DateTimeFormat({dateStyle:''medium'', timeStyle:''short'', timeZone: effectiveTz}) with the zone appended so it''s unambiguous. (3) Conversion helpers need explicit tests for DST spring-forward (nonexistent wall time), fall-back (ambiguous), a non-integer-offset zone (Asia/Kolkata +05:30), the date line (Pacific/Auckland), and Z-in/offset-out round-trip. Hard-to-write tests here are a signal Intl-only (RR-35VJ8G) is safer.'
severity: minor
resolution: 'Accepted into plan. (1) RandomDatetime() arm in fixtures.go is required (unknown types fall through to RandomString and would feed invalid values). (2) Display format pinned: Intl.DateTimeFormat({dateStyle:''medium'', timeStyle:''short'', timeZone: effectiveTz}) with the zone appended. (3) Conversion helper tests required for: DST spring-forward, fall-back, Asia/Kolkata +05:30, Pacific/Auckland date-line, and Z-in/offset-out round-trip. These become AC test scenarios.'
status: addressed
---
