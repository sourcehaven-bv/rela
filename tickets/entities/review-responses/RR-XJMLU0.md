---
id: RR-XJMLU0
type: review-response
title: SQL ListSessionsDue fails the whole round on one damaged row, unlike filechannels
finding: 'sqlchannels.ListSessionsDue (sql.go ~1193) collects with scanChannel. scanChannel returns an error for a damaged created_by or session_restarts JSON (sql.go:236-247), so one bad row fails the listing for every timer type, every round, and stops all timers on sqlite and postgres. filechannels skips a damaged meta.yaml with a warning (file.go visitTypesAt), and the plan made that change for exactly this reason (DR minor ''one damaged meta.yaml stops all timers''). Fix: in ListSessionsDue, scan rows one by one, log and skip a row whose decode fails (key columns are still readable), and add a conformance or sqlchannels test with a corrupted session_restarts value.'
severity: minor
resolution: SQL ListSessionsDue logs (rate-limited) and skips a damaged row; channelstest.RunDamagedDueRow runs on sqlite and postgres.
status: addressed
---
