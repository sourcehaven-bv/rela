---
id: BUG-WS7QF2K
type: bug
title: Read-only record actions, ways out of a closed workspace, banner placement, people list and dark mail body
description: A browser test of a SaaS setup on rela found that a read-only workspace still offered Edit, Duplicate and Delete, a suspended or archived workspace page had no way out, the read-only banner was cut off and squeezed, a member was missing from the people list until their first change, and a dark mail kept a light body.
priority: medium
effort: m
why1: The read-only rule hid only create verbs, Duplicate and conversions do not read _actions, the workspace pages had no links, the banner sat in the content pane, provisioning ran only on a write, and the mail dark block styled .wrap but not body.
why2: BUG-9DR9867 and BUG-RO4WSP1 treated read-only as "no create" and the workspace pages as final states, and placed the banner where page cards go.
why3: Each state was checked for its wording, not walked through as a user who wants to leave the page or keep working.
why4: No test pinned the record menu in a read-only workspace, the exits of the workspace pages, or where the banner sits; the mail compat test checked that the dark block exists, not what it paints.
why5: Customer-facing states are added one screen at a time, with no checklist of what a user can still do and how they leave.
prevention: Vitest tests pin that every write verb is hidden in a read-only workspace, the record menu without edit, duplicate, delete and convert, the account and sign-out links and contact text of the workspace pages, the banner slot of the shell, and the provisioning POST. Go tests pin the shell values, the config keys, POST _me provisioning and the dark body rule.
started: "2026-11-22"
completed: "2026-11-22"
status: done
---

## Description

Findings 4, 5, 7, 8 and 13 of a browser test of a SaaS setup built on rela.

- In a read-only workspace, a record's "..." menu still offered Edit,
  Duplicate and Delete.
- A suspended or archived workspace page had no sign out, no link to the
  account portal and no logo, and its contact text could not match the
  portal.
- The read-only banner sat in the list column: the sticky table header cut it
  off, and an open side panel squeezed it.
- The people list said "No people yet" while the owner was signed in, until
  their first change. With `unmatched_principal: provision`, a person is
  created on the first write, so a member who only reads is never listed.
  Clarifying the empty text would describe that gap rather than close it; a
  member who has only looked around is still a member. The fix provisions on
  an explicit `POST /_me` when the app opens, so the provisioning rule stays
  "never on a GET".
- A mail from `mail.render` in dark mode turned only `.wrap` dark; the
  body kept #f4f5f7. The golden delta is that one rule.

## Acceptance Criteria

- [x] A read-only workspace hides Edit, Duplicate, Delete, Rename, copy and convert on a record, inline editing and row Delete.
- [x] The suspended, archived and unknown workspace pages show the logo (`app.logo`) or the app name, a link to `account.account` and Sign out (`account.sign_out`) when configured.
- [x] `account.contact` replaces the default contact text on those pages.
- [x] The read-only banner spans the full width below the page header, above the content and the side panel.
- [x] Opening the app provisions a provisionable member, so the people list shows them; a GET never provisions, and a read-only workspace refuses the POST.
- [x] A dark-mode mail paints the body with the dark background.
- [x] The behaviour is documented.
