---
id: AM-fsstore-body-no-conflict-marker
type: automated-measure
title: An fsstore write never produces a file that reads as conflicted
description: Planned for BUG-GYM7ZBS. Writes an entity whose body has a line starting with <<<<<<< and asserts the row reads back without ErrConflictedRow.
kind: test
location: internal/store/fsstore (planned)
status: proposed
---
