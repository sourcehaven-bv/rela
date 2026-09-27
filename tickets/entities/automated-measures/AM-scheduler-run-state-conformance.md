---
id: AM-scheduler-run-state-conformance
type: automated-measure
title: Scheduler run-state backends honour one run lifecycle contract
description: 'Runs the schedulerstatetest conformance suite on kvstate and pgschedstate: one active run per task; compare-and-set start; lease reap; once-only child settlement; prune.'
kind: test
location: internal/schedulerstate/schedulerstatetest
status: active
---
