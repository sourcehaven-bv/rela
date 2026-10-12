---
id: RR-RSVJSW
type: review-response
title: Rename tombstone has no undo
finding: BeforeRename tombstones channels on newID before the Tx; a Tx failure leaves them detached, and an entity created in the free()/Tombstone gap loses its first channel.
severity: minor
status: open
---
