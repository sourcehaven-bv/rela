---
id: RR-RYKBHZ
type: review-response
title: 'Design: every process that runs a converter needs the variable'
finding: The variable is read per process. The guide sets it only in the rela-server unit. rela render, scheduler or CD-run rela invocations read their own environment and run with system dirs only unless they get it too. The atlas devops change should use the shared EnvironmentFile; docs should say every converter-running process needs it.
severity: minor
resolution: docs/transforms.md states the variable is per-process and must be set for every process that runs a converter (server unit, shell, timer, deploy script), recommending a shared EnvironmentFile. On atlas the devops change writes it into /etc/atlas/env, the EnvironmentFile of the rela-server unit, which also runs the scheduler in-process; operator shells source the same file.
status: addressed
---
