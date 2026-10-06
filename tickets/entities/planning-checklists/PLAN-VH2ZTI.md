---
id: PLAN-VH2ZTI
type: planning-checklist
title: 'Planning: Add --access-log for per-request timing at info level'
started: "2026-10-05"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** see TKT-DFUAT5 § Scope. Planning was written after a first
implementation (a bool flag logging to the application log); this plan is the
redesign after the question whether interleaving access lines with the app log
makes sense.

**Acceptance Criteria:** TKT-DFUAT5 § Acceptance. Each maps to a test in § Test
Plan.

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: small change, one flag and a logger)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**

- `log/syslog` (stdlib, frozen but stable) as an `io.Writer` behind a
`slog.TextHandler`. No new dependency.
- Rejected: `coreos/go-systemd/journal` (native journal fields, but a new
dependency and Linux-only; syslog gives the identifier, which is all the
consumer filters on).
- Codebase: `internal/dataentry/requeststats.go` already builds the record
and the per-request `store.QueryStats`; `App.SetJWTGate` is the pattern for
router configuration set before `NewRouter`.
- Reference: nginx `access_log syslog:server=unix:/dev/log,tag=...`, which
the devops fleet already uses (`nginx_perf`).

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** `requestStats(next, accessLog *slog.Logger)`. With an
access logger the record goes there at Info (once, also under Debug); without
one, the Debug record on the default logger as before.
`App.SetAccessLog(*slog.Logger)`. `cmd/rela-server` parses `--access-log` (`""`,
`stderr`, `syslog`), validates in `parseFlags`, and builds the logger in
`accesslog.go`; the syslog dial is behind a build tag.

Alternatives rejected:

- Bool flag into the application log (first implementation): one line per
request buries warnings and errors; the only split is a regex on text.
- Log file: needs its own rotation and is not shipped by journal pulling.
- stdout vs stderr: journald gives both the same identifier.

**Files to modify:** `internal/dataentry/{requeststats,app,router}.go`,
`cmd/rela-server/{main,accesslog,accesslog_syslog,accesslog_nosyslog}.go`,
tests, `.golangci.yml` (allow `log/syslog`), GUIDE-server-security.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** the flag value, allowlisted to `stderr` and
`syslog`; anything else exits 2. Request method and path are logged; the text
handler quotes values, so a path cannot inject a fake key or a new line.

**Security-Sensitive Operations:** what leaves the process. No query string,
user, IP or body; no SQL. The query count goes only to the log, never to the
client (RR-64OR7D).

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

- Access logger at Info, no header, no query string, app log empty
(`AccessLogAtInfoWithoutHeader`).
- Under Warn the access log still writes (`AccessLogIgnoresQuiet`).
- Debug plus access log: header as before, one record, only in the access
log (`AccessLogUnderDebugLogsOnce`).
- Through `NewRouter` (`AccessLogWiredIntoRouter`).
- Syslog line has no time/level (`SyslogAccessLogger_DropsTimeAndLevel`).

**Edge Cases:** SSE (record on disconnect, no header); Windows (no syslog:
startup error, build still compiles); systemd `PrivateDevices=true` (`/dev/log`
checked present inside the atlas unit's namespace).

**Negative Tests:** unknown destinations rejected at flag parsing (exit 2) and
by the constructor; an empty value means off at flag level
(`CheckAccessLogDest`, `NewAccessLogger_RejectsUnknownDestination`).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** syslog socket missing in a sandbox: startup fails loudly rather than
logging nothing. journald rate limit shared with the app log, and synchronous
writes: documented (RR-AB6R1J, RR-UKZV3Z). An older rela exits on the unknown
flag: the devops side turns it on together with the pin bump. Effort: s.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** GUIDE-server-security (new section "Access log"),
generated `docs/server-security.md`, flag help text.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** 11 findings, all addressed: RR-AB6R1J (significant:
journald rate limit is per unit, documented), RR-090JDW, RR-YD6YFX, RR-Q51NXA,
RR-9IP4A3, RR-QE8IZ7, RR-UKZV3Z, RR-X8U03I, RR-T01UDR, RR-HDT0R2, RR-CYN839.
Changes from the review: record written from a defer (panic=true), path capped
at 512 bytes, access log opened before the project loads (exit 1), Debug writes
the record to both loggers, one allowlist, guide section on limits and path
content.
