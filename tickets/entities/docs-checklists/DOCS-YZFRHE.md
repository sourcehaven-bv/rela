---
id: DOCS-YZFRHE
type: docs-checklist
title: 'Documentation: Add --access-log for per-request timing to syslog or stderr'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious (requestStats: separate sink, Debug to both, defer and the recorded 500, 1xx handling; accesslog.go: which function owns which check, why time/level are dropped for syslog)
- [x] Function/type docs if public API (dataentry.WithAccessLog)

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: server flags are documented in GUIDE-server-security, not the README)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new pattern; the depguard allow for log/syslog is commented in .golangci.yml)
- [x] Help text accurate (if CLI changes) (`--access-log` help lists =stderr and =syslog and the fields)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: repo keeps no CHANGELOG; releases come from commit history)
- [x] API docs updated (if applicable) (GUIDE-server-security: new section "Access log (`--access-log`)", regenerated docs/server-security.md)
