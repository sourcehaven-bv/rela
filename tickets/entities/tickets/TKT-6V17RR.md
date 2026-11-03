---
id: TKT-6V17RR
type: ticket
title: Configure editor for dashboard card width
kind: enhancement
priority: low
status: backlog
---

## Description

TKT-H21TYS added `width: normal | wide | full` to dashboard cards. The key is
locked in `internal/configedit/allowlist.go` (`dashboard.cards.[].width`,
`dashboards.*.cards.[].width`), so an operator writes it in `data-entry.yaml`.
Editing a card in Configure keeps the width it has.

Add it to the Configure Dashboard screen:

- a segmented control (Normal, Wide, Full) in the card editor, for query and
list cards alike;
- writing `normal` removes the key rather than storing the default;
- unlock the two keys in the allowlist (`width` is not protective: it
restricts nothing).

Labels go through i18n keys in `en` and `nl`.

Out of scope: how the dashboard lays cards out.
