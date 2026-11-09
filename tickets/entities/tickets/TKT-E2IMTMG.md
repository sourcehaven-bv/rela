---
id: TKT-E2IMTMG
type: ticket
title: 'rela test v1 (4/4): tickets/ sample suite, CI step and guide'
kind: enhancement
priority: medium
effort: m
status: backlog
---

## Description

Part 4 of TKT-0MCLX9S.

- Add `tickets/tests/`: checklist auto-creation on a status change, a validation rule and an access expectation. The suite uses generated data only, with no literal timestamps.
- Add a CI step that runs `rela test --project=tickets`.
- Add a docs-project guide, "Testing your app with rela test": generated data and `--seed`, principals and roles, given-state fixtures, memstore differences, and what is not reproducible.
- Update the CLI reference and add a link from lua-scripting.
- Mark the umbrella TKT-0MCLX9S done.
