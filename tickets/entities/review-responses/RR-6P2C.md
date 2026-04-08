---
id: RR-6P2C
type: review-response
title: Range filters (lt+gt on same property) not representable
finding: 'Plan says last-write-wins for filter[due_date][lt]=...&filter[due_date][gt]=... A date range is a legitimate user request. Either support {ops: [{op:''gt'',v:...},{op:''lt'',v:...}]} or document range filtering as out of scope explicitly.'
severity: minor
reason: Range filters (lt+gt on same property) require a multi-op data shape (Record<string, FilterValue[]>) that's a bigger refactor. v1 keeps last-write-wins for the same property, documented in user guide. Follow-up ticket if there's demand.
status: deferred
---
