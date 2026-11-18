---
id: DOCS-9IB92XB
type: docs-checklist
title: 'Docs: in-transaction audit records'
status: done
---

## Code Documentation

- [x] Comments where logic isn't obvious (auditTx, emitWrite, txBatch and heldAudit explain when a record joins the transaction and when it is held until commit)
- [x] Function/type docs if public API (TxAudit, sqlaudit.Log.AppendAll, sqliteaudit.NewInTx and pgstore.TxHandleFor are documented)

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no README content covers audit delivery)
- [x] CLAUDE.md updated (if new patterns) (internal/entitymanager/CLAUDE.md: audit goes on the write's view through auditTx/emitWrite; intro names the database log)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI changes)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repo keeps no changelog; release notes come from PR titles)
- [x] docs/audit-log.md Delivery section rewritten; docs/content-states.md copy sentence updated
