---
id: RR-Q7M1DK
type: review-response
title: '`rela validate` does not run the run_as grant check unless data-entry.yaml has channels:'
finding: 'internal/cli/validate.go:586 returns early when !projectsetup.HasChannels(data), and also when an earlier check failed, so realtime.yaml (and with it the grant check) is never loaded by `rela validate` in most projects. The guide (GUIDE-realtime.md:919, ''The same check runs at boot, in `rela validate`, ...''), acl-security.md (''The check runs everywhere the file loads'') and the plan''s risk mitigation for strict loading (''rela validate reports it before a deploy'') all rely on it. TestRunAsGrantCheck_StrictEverywhere''s ''validate and configure'' subtest calls appbuild.LoadRealtime directly, so it cannot catch this. Fix: have `rela validate` load realtime.yaml whenever the file exists (report a ✗ line on failure), independent of the channels: section, and drive the subtest through reportChannelsValidation (or the command) rather than LoadRealtime. Otherwise correct the docs.'
severity: significant
resolution: rela validate loads realtime.yaml whenever it exists and runs the run_as grant check; tested through ValidateCmd.Run with ungranted and granted acl.yaml; guide updated.
status: addressed
---
