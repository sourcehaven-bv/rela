---
id: TKT-TU0VZ4Y
type: ticket
title: Record entity and relation audit rows inside the write transaction (fail closed)
kind: enhancement
priority: high
effort: l
tags: security
started: "2026-11-18"
completed: "2026-11-18"
status: done
---

## Description

PR 2 of FEAT-0BH46JQ. Depends on TKT-USQBMG6.

On postgres and sqlite the entity manager writes each entity and relation audit
record on the store transaction view of the write it records, so a failed append
rolls the write back. Writes that ran without a transaction open one when the
sink is transactional. Reverses the best-effort rule; see the decision entity.
