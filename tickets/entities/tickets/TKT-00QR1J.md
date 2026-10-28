---
id: TKT-00QR1J
type: ticket
title: Channel composer @ mention picker
kind: enhancement
priority: medium
effort: m
status: backlog
---

## Description

Add an `@` picker to the markdown fields of `RlChannelComposer` (combobox ARIA,
keyboard and pointer, i18n en and nl) that inserts `` `USR-7` ``, ``
`integration:x` `` or `` `@login` ``, with a debounced gated `searchEntities `
call for users the poster can read.

Add the `mentions ` block (`user_entity_type ` from `MentionUserType() `,
`login_form `, `jobs `) to `GET /api/v1/_schema/channels `, the same for
every principal.

Without person mapping, offer authors seen in held messages by form (job
identities as `` `integration:x` ``, never `unknown ` or rela identities); this
was AC 16 and AC 17 of TKT-6RXAC0's draft plan.

Follows TKT-6RXAC0.
