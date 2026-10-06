---
id: IMPL-ZKS730
type: implementation-checklist
title: 'Implementation: Add --access-log for per-request timing to syslog or stderr'
started: "2026-10-05"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: middleware tests use httptest requests and an in-memory slog handler; the router test uses the existing newTestAppV1 fixture)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (N/A: assertions are on log text, which is the contract)

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

- `rela-server --access-log=bogus`: "want stderr or syslog", exit 2.
- `rela-server --quiet --access-log=stderr` on prototypes/data-entry/project:
one `level=INFO msg=request` line per request; `?token=secret` absent.
- `--quiet --access-log=syslog`: zero request lines on stderr. macOS does
not retain these syslog datagrams in its unified log, so the sink was verified
on Linux instead: a probe with the same `log/syslog` writer and handler options,
run on atlas (Debian, journald) in a transient unit with `PrivateDevices=yes`,
produced SYSLOG_IDENTIFIER=rela-access-probe, MESSAGE `msg=request method=GET
path=/api/v1/probe status=200 ...`, attributed to the transient unit (confirms
the per-unit rate limit, RR-AB6R1J). Probe binary removed afterwards.
- `/dev/log` present inside the running atlas rela unit's mount namespace.
- `GOOS=windows go vet ./cmd/rela-server`: builds with the no-syslog stub.
- Unit tests: AccessLogAtInfoWithoutHeader, AccessLogIgnoresQuiet,
AccessLogUnderDebugLogsToBoth, AccessLogRecordsPanic,
AccessLogTruncatesLongPath, AccessLogWiredIntoRouter (integration through
NewRouter), CheckAccessLogDest, NewAccessLogger_*,
SyslogAccessLogger_DropsTimeAndLevel.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — one allowlist (checkAccessLogDest), one attrs builder for both sinks
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned) — startup errors exit; runtime syslog write errors are dropped by slog, accepted and documented (RR-CYN839)
- [x] No debug code left behind
