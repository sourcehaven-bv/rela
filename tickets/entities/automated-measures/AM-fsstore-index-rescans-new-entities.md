---
id: AM-fsstore-index-rescans-new-entities
type: automated-measure
title: An entity created after the fsstore index was saved is found on the next open
description: Saves the fsstore index, creates an entity file whose directory mtime equals the saved one, reopens the store and asserts the entity is listed. Fails if the index trusts the directory mtime alone.
kind: test
location: internal/store/fsstore (planned with BUG-TSX9CO)
status: proposed
---
