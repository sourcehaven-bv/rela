---
id: MEAS-RRULE-NEXT-ANCHOR
type: automated-measure
title: Lua rrule_next anchors a rule without DTSTART at the after date
description: Lua tests pin that rela.rrule_next returns a date later than after when after is an occurrence of a rule without DTSTART, including today, and with DTSTART in both forms.
kind: test
location: internal/lua/date_test.go
status: active
---
