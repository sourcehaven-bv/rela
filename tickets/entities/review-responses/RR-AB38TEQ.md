---
id: RR-AB38TEQ
type: review-response
title: Nil manager branch in elevatedUpdatesFor
finding: elevatedUpdatesFor returned nil quietly for a nil manager.
severity: minor
resolution: The branch is removed. NewApp already rejects a nil manager and NewScopedUpdates rejects it too.
status: addressed
---
