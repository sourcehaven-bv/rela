---
id: TKT-JEAR2UA
type: ticket
title: Root opens the space home; config for built-in sidebar rows
kind: enhancement
priority: medium
effort: m
started: "2026-11-18"
completed: "2026-11-18"
status: done
---

## Description

Two gaps found while configuring a customer portal (FEAT-XT3SOP spaces).

1. The app root `/` always redirected to `/dashboard`, so a space's `home:`
applied only at `/s/<space>`. A customer who opens the app root must land on the
home of the space they may use. The root now opens the home of the current
space: the space the user is in, else the one this browser was last in, else the
first space the principal may enter. Without `spaces:` the root still opens
`/dashboard`. `/dashboard` keeps working.

2. Search, Inbox, Flow tasks, Flow runs and Analysis are pinned sidebar rows
with no config. A new `builtins:` block in data-entry.yaml, per space and at the
top level without spaces, hides a row (`hidden: true`) or shows it only to
holders of a global permission (`permission:`). Unknown row names are refused at
config load. Like a navigation entry's `permission:`, this is a UX filter: the
pages behind the rows stay reachable by URL, and the config is not a secret.

```yaml
spaces:
  - id: portal
    label: My account
    permission: portal.use
    builtins:
      search: {hidden: true}
      flow_tasks: {hidden: true}
      flow_runs: {hidden: true}
      analysis: {hidden: true}
```

## Acceptance criteria

- `/` redirects to the current space's home with spaces, `/dashboard` without.
- The last used space is remembered per project base in localStorage.
- `builtins:` hides or gates each built-in row per space and at the top level.
- Unknown row names, both or neither of hidden/permission, and a top-level
block together with `spaces:` are load errors.
- Default unchanged: every row shown.
