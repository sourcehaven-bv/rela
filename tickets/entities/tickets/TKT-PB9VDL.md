---
id: TKT-PB9VDL
type: ticket
title: 'EntityDetail: split the loaded view into a child component'
kind: refactor
priority: low
status: backlog
---

## Description

`frontend/src/components/entity/EntityDetail.vue` is about 2700 lines. Its
loading, error, inaccessible and loaded states are spread across separate flags:
`loading`, `error`, `entry`, `viewData`, `isInaccessible` and `worldAbsent`.
Every consumer has to reason about which combination it runs in.

Proposal: the parent handles load and error only. Once the view has loaded it
renders `<EntityDetailLoaded :entry>`, where `entry` is a required prop. The
face address is then always `entityRef(entry)` and needs no null guard. The
loaded state, autosave and the modals move into the child.

This supersedes the null guards added for `servedRef`, which is now `string |
null` and null until the entry loads (RR-RN3YGR). With the split, `servedRef`
becomes non-null by construction and those guards go away.
