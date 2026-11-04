---
id: TKT-HHHU76J
type: ticket
title: Document a draft-then-complete protocol for fs and memory
kind: docs
priority: medium
effort: xs
status: backlog
---

## Description

Document a draft-then-complete protocol for backends without rollback (fs,
memory), where `POST /api/v1/_batch` answers 501 (RES-QKREXZZ option 3): create
entities with a draft marker, link them, flip the marker last, and filter
readers on it. Use caller-chosen ids so a retry is idempotent.
