---
id: RR-LVJZ6T
type: review-response
title: 'Alternative not considered: decide under the store lock'
finding: Guard evaluation is pure CPU (message props, session, standard library; no entity, no I/O). Append could take a decision callback, func(cur Session) (to string), evaluated inside the existing lock or transaction. That removes the head re-read, the retry loop, the 409 path for posts and the always-conflicting wrapper test, and the "lowest seq wins" rule then holds trivially. The CLAUDE.md Tx rule forbids slow I/O in the callback, not pure evaluation. Either adopt it or record why it was rejected (store stays policy-free, determinism across backends).
severity: minor
resolution: 'Plan updated: adopted deciding under the store lock with a pure decider whose role answers are computed beforehand; the rejected retry alternative is recorded.'
status: addressed
---
