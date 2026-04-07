---
id: RR-F60M
type: review-response
title: $today uses local time, breaks across timezones
finding: time.Now() is local time. When server runs in UTC and user is in another zone, $today shifts by up to 24 hours from user expectations. Test uses time.Now() in both code and assertion, so it's tautological and can't catch the bug.
severity: critical
resolution: Replaced time.Now() with nowFunc() that defaults to time.Now().UTC(). Tests pin nowFunc to a fixed UTC timestamp so $today substitution is deterministic. Documented in godoc that variables resolve in UTC.
status: addressed
---
