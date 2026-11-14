---
id: TKT-EBBHM55
type: ticket
title: Push desktop notification conditions into the store
kind: enhancement
priority: medium
effort: s
status: backlog
---

## Description

TKT-FED6FMM left `internal/desktopnotify/evaluate.go#Config.Evaluate` as a
`request-path` entry in `internal/archguard/unpagedread_allowlist_test.go`.
Every badge poll reads every header of each type a notification rule names and
evaluates the condition in Go. Lower each rule condition with
`queryplan.LowerScope` where it lowers exactly, read the matches with
`GraphQueryHeaders` and a limit, and count the badge with `store.CountMatched`.
Pin it with `storetest.AssertScaleFree`.
