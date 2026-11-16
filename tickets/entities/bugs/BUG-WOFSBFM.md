---
id: BUG-WOFSBFM
type: bug
title: Deleting a ledger owner removes one side of its transactions
description: Deleting an entity that owns ledger rows drops those rows; the other legs of the same transactions stay, so the other owners no longer balance.
priority: medium
effort: s
status: backlog
---

## Description

Deleting an entity that owns ledger rows drops those rows (TKT-J7Q9C4K). The
other legs of the same transactions stay, so the other owners no longer balance.
GUIDE-records tells operators to archive such accounts instead of deleting them.

## Expected

Deleting an owner that still has ledger rows is refused, with a message that
names the record type, unless the delete is a purge that removes the whole
ledger.
