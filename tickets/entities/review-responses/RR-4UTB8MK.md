---
id: RR-4UTB8MK
type: review-response
title: Constructor accepts a rollback store with a best-effort sink
finding: New does not reject a database store paired with a sink that is not a TxAudit.
severity: minor
reason: appbuild always pairs them; tests deliberately pair a memory sink with sqlite/pg. auditTx holds records until commit, so no rolled-back write is recorded either way.
status: deferred
---

New does not reject a database store paired with a sink that is not a TxAudit.
