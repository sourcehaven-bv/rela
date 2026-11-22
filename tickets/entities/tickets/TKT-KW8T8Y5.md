---
id: TKT-KW8T8Y5
type: ticket
title: Detail action order and danger style; hide Shortcuts on phones
kind: enhancement
priority: medium
effort: s
started: "2026-11-22"
completed: "2026-11-22"
status: done
---

## Description

Found while configuring the harbourmaster customer portal.

1. The entity page sorts its action buttons (actions with `available_on:`) by
label, so "Archive" comes first among the tenant actions, and a destructive
action looks like every other button. An operator cannot set the order or mark
an action as destructive.
2. The sidebar footer always shows the Shortcuts button, also on a phone, where
keyboard shortcuts are of little use. An operator cannot hide it.

## Proposal

- `style: danger` and `order: <int>` on an action in data-entry.yaml. The
entity page sorts its actions by order (absent is 0), then label, and draws a
danger action like Delete. `/_config` already serves the action verbatim, so
both keys reach the SPA without a change to `_actions`.
- A `shortcuts` row in `builtins:` for the footer Shortcuts button, and a
`hidden_on_phone: true` field on every `builtins:` entry. The sidebar endpoint
names those rows in `phone_hidden_builtins`; the SPA leaves them out below 768
pixels.

Defaults stay as today. Actions are locked in Configure (`actions.**`), so no
Configure model changes.

## Acceptance Criteria

1. Without `order:`, detail actions sort by label as before; with it, lower
first, absent counts as 0, ties by label then id. Test: detailActions.test.ts.
2. `style: danger` draws the desktop button and the overflow menu entry as
danger. Test: EntityDetail.actions.test.ts.
3. A `style:` other than `danger` and a fractional `order:` fail config load.
Test: TestValidateActions_Style, TestAction_OrderRejectsFraction.
4. `builtins.shortcuts: {hidden: true}` hides the Shortcuts button;
`{hidden_on_phone: true}` hides it only on a phone-sized screen; no entry shows
it everywhere. Test: Sidebar.builtins.test.ts.
5. `hidden_on_phone` reaches `/_sidebar` as `phone_hidden_builtins`, per space,
for every principal. Test: TestSidebarBuiltins_HiddenOnPhone,
TestSidebarBuiltins_HiddenOnPhoneInSpace.
6. `hidden` with `hidden_on_phone` and an empty entry fail config load. Test:
TestValidateBuiltins.
