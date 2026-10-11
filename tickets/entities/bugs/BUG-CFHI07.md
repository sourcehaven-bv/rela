---
id: BUG-CFHI07
type: bug
title: rela rename id records no principal
description: RenameIDCmd calls RenameEntity with context.Background(), so the audit record and the synchronous rename version carry no principal and history shows 'unknown (unknown)'. Other CLI writes stamp the CLI principal.
priority: low
status: backlog
---
