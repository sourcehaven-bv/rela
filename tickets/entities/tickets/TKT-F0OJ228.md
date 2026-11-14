---
id: TKT-F0OJ228
type: ticket
title: 'ACL guide: adopted row on a push remote is deleted when gone'
kind: docs
priority: low
effort: xs
started: "2026-11-14"
completed: "2026-11-14"
status: done
---

## Description

GitHub issue #2229. The ACL guide said that on a remote with `push`, a row the
pull adopted follows the conflict rule when `on_gone: delete` applies. The code
(`internal/remotesync/gone.go`) deletes an adopted, unedited row like any other
linked row; only a row edited here since the last sync follows the conflict
rule. `GUIDE-remote-sync.md` already describes this correctly.

## Fix

Correct the sentence in `GUIDE-acl-security.md` and regenerate
`docs/acl-security.md`.
