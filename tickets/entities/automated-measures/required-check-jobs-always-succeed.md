---
id: required-check-jobs-always-succeed
type: automated-measure
title: Required check jobs always run and report success
kind: ci
location: .github/workflows/ci.yml (rela-tickets job)
status: active
description: |
  Required-check jobs must always execute and finalize as 'success', never as
  'skipped'. GitHub's auto-merge disables itself when any required check on the
  head commit resolves to a non-success conclusion (including 'skipped'). Use
  a first step that sets steps.gate.outputs.applies, then gate every other step
  with 'if: steps.gate.outputs.applies == true'.
---

Required-check jobs in GitHub Actions workflows must always execute and finalize
as 'success', not 'skipped'. GitHub's auto-merge feature disables itself when
any required check on the head commit ends in a non-success conclusion
(including 'skipped'). Pattern: replace any job-level 'if:' with a first step
that sets steps.gate.outputs.applies, and gate downstream steps with 'if:
steps.gate.outputs.applies == true'. The job then always runs and reports
success, even when the underlying gate does not apply.
