---
id: BUG-KABQT1
type: bug
title: Unguarded copies write mapped fields without the field write check
description: An unguarded copy is authorized by the row grant on the target only; its field mappings then write target properties with no field-grant check, so a mapped read-only field can be set by editing the source and invoking the copy.
priority: medium
status: backlog
---

Found in the TKT-0XL8MF security review (copy.go buildCopyTarget,
authorizeCopy). Pre-existing.
