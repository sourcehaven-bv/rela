---
id: BUG-NFCPUW
type: bug
title: History snapshots serve properties that have left the schema
description: Version snapshots keep properties later removed from the schema; the historical closed world only denies declared fields, so such a property is served unredacted by the HTTP history API and rela.get_version.
priority: medium
status: backlog
---

## Description

A version snapshot keeps every property it was captured with, forever. The
historical closed world (affordances resolver, `declaredFields`) only denies
fields the schema still declares, so a property removed from the schema and from
its `visible:` block passes through unredacted on the HTTP history API and on
`rela.get_version`.

Found in the TKT-EC7F65 code review. Pre-existing; not introduced there.

## Fix direction

In history, deny every snapshot property that is not declared AND affirmatively
granted.
