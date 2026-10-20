---
id: RR-VQ4L44
type: review-response
title: rela validate builds the whole service, and aborts on a broken realtime.yaml
finding: 'internal/cli/validate.go:568-603 reportChannelsValidation calls appbuild.Discover (store load, search index, a DB connection on postgres builds) only to get the realtime schema. That is the third possible Discover in one `rela validate` run (see lines 106 and 141). A Discover failure, which includes an invalid realtime.yaml, returns an error from Run. That aborts every later check (mail templates, schedules, connections, classification) and prints an infrastructure error instead of a ✗ line. cmd/rela-server/configure.go already does this cheaply with appbuild.LoadRealtime(ctx, cfg, meta, policy). Use the same here: parse the metamodel and policy, LoadRealtime, report its error as a ✗ line, then CheckChannels. Drop the unused `_ context.Context` parameter and the nolint it forces. Also, the plan (section 13 D2, AC 2) says `rela analyze`, but this is wired into `rela validate`. Fix the plan or the docs so they agree.'
severity: significant
resolution: rela validate now loads realtime.yaml through a small loader (acl policy, FS config loader, LoadRealtime) without Discover and prints a cross mark line instead of aborting when realtime.yaml does not load.
status: addressed
---
