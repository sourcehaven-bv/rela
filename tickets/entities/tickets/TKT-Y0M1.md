---
id: TKT-Y0M1
type: ticket
title: Add date arithmetic and RRULE helpers to Lua runtime
kind: enhancement
priority: medium
effort: s
status: ready
---

## Description\n\nAdd four Lua date helper functions for recurrence scheduling:\n\n- `rela.date_add(date, offset)` — add d/w/m/y offsets (negative supported)\n- `rela.date_weekday(date)` — get lowercase weekday name\n- `rela.date_next_weekday(date, day)` — next occurrence of weekday\n- `rela.rrule_next(rrule, after?)` — next RRULE occurrence using teambition/rrule-go\n\nUses RFC 5545 RRULE syntax for iCal recurrence rules.\n\n## Implementation\n\n- New file: `internal/lua/date.go` with four standalone functions + `registerDateHelpers`\n- New file: `internal/lua/date_test.go` with comprehensive tests\n- Modified: `internal/lua/runtime.go` — register helpers, refactor `luaDaysSince` to use shared `parseDate()`\n- New dependency: `github.com/teambition/rrule-go v1.8.2`\n- Updated: `.go-arch-lint.yml` — allow rrule-go in lua component\n\nPR: #297"
