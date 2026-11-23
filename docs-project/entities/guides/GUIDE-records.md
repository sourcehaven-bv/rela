---
id: GUIDE-records
type: guide
title: Records (time series and ledgers)
status: published
order: 32
audience: advanced
summary: "Declare record types in schema.yaml to store meter readings, events and ledger postings owned by an entity, outside the graph"
---

A record is a typed row with a timestamp, owned by one entity. Records suit
data that does not fit the graph: meter readings, measurements, events and
ledger postings. A meter can hold thousands of readings without each one
becoming an entity with relations.

Records are append-only by default. A record type with `mutable: true`
lets you change or delete one row at a time, which suits data such as hour
registration. A ledger is never mutable: it corrects a posting by appending
its reversal.

## Declaring record types

Record types live under `records:` in `schema.yaml`, next to `entities:`.

```yaml
entities:
  account:
    label: Account
    id_prefix: "ACC-"
    properties:
      name: {type: string}
      locked_until: {type: date}
  meter:
    label: Meter
    id_prefix: "MTR-"
    properties:
      name: {type: string}

records:
  reading:
    owner: meter
    retention: 400d
    fields:
      kwh: {type: decimal, scale: 2, required: true}
  posting:
    owner: account
    ledger:
      amount: amount
      group_by: txn
      commodity: currency
      closed_before: locked_until
    fields:
      amount: {type: decimal, scale: 2, required: true}
      txn: {type: string, required: true}
      currency: {type: string, required: true}
```

| Key | Meaning |
| --- | --- |
| `owner` | The entity type that owns each row. Required. |
| `fields` | The row's fields. The types are `string`, `integer`, `decimal`, `boolean`, `date`, `datetime` and enums. Lists, computed, rollup and unique fields are not allowed. |
| `retention` | How long rows are kept, such as `90d` or `24h`. Without it, rows are kept for ever. A ledger cannot have a retention. |
| `ledger` | Makes the type a double-entry ledger. See below. |
| `mutable` | Lets a row be changed or deleted. See "Changing rows" below. A ledger cannot be mutable. |
| `permissions` | Optional `read` and `append` permission names. A caller needs the permission and the usual owner access. `read` covers list, aggregate, roll-ups and ledger transactions. Reversing a transaction needs both. `append` covers append and changing or deleting a row. A role grants the permission in `acl.yaml`; it is held globally. |

A record type cannot share its key with an entity type. The field names
`id`, `type`, `owner`, `ts`, `recorded_at`, `recorded_by`, `group`,
`reverses`, `updated_at`, `updated_by` and `revision` are reserved.

## Ledgers

A ledger type groups rows into transactions by the `group_by` field. Each
transaction must balance: its `amount` values add up to zero, per
`commodity` when one is named. All rows of a transaction have the same `ts`.
rela refuses a transaction that does not balance, and refuses a second
transaction with a group key that is already used.

`closed_before` names a date property on the owner. A posting dated on or
before that date is refused, so a closed period stays closed. rela checks the
balance and the group key first, so an unbalanced posting or a reused key never
tells you whether a period is closed.

To correct a transaction, reverse it. A reversal appends the same rows with
the amounts negated. Each transaction can be reversed once.

## Changing rows

A row of a mutable type can be changed or deleted. You need the same right
as for appending: you may update every face of the owner. A row has no
owner of its own beyond its entity, so anyone who may append to an owner may
also change any of its rows. When rows need per-person rights, give each
person their own owner entity.

A change can set new field values and a new `ts`. The owner of a row never
changes. A field you do not name keeps its value; `null` clears an optional
field.

Each row carries a `revision`, which starts at 0 and goes up by one on every
change. A change or delete names the revision it read. When the row changed
in the meantime, rela refuses the request with 409, so two people editing
the same row cannot overwrite each other unnoticed.

A changed row records who changed it (`updated_by`) and when
(`updated_at`). Every change and delete also writes a line to the audit log
(`update-record` and `delete-record`). The line names who changed which
fields, but not the values: earlier values are not kept. A change refused
because you may read the owner but not update it writes a `denied-write`
line.

When a field is removed from the schema, a change drops it from the row.

Reads, aggregates and retention work the same on mutable types.

## Access control

Records have no grants of their own. They follow their owner:

- You can read an owner's rows when you can read every face of the owner.
  Rows belong to the whole owner, not to one face. To anyone else the
  owner's rows do not exist, and a request for them answers 404.
- You can append rows when you may update every face of the owner.
- An aggregate folds only the rows of owners you can read.
- You can see a ledger transaction only when you can read every owner in
  it.

A record type therefore has its owner type's classification. When records
need stricter rules than their owner, such as a salary ledger on a person,
declare a separate owner type with those rules (for example a
`salary-account` related to the person) and put the record type on it.

A group key is unique across the whole ledger type. When you append a
transaction whose key is already used, rela answers 409, even when the
existing transaction belongs to owners you cannot read. So a caller who
may post can learn that a key is in use. Use keys that reveal nothing on
their own, or keys the posting application generates.

Each row stores who appended it (`recorded_by`) and when
(`recorded_at`). Every append also writes one `append-records` line to the
audit log, however many rows it holds. The line names the record type, the
row count, the first and last row id, the owners and, for a ledger, the
transactions. A reversal writes the same line, starting with the
transaction it reverses. An append refused because you may read an owner
but not update it writes a `denied-write` line that names the owner. No
other refusal writes a line: not a hidden owner, and not an invalid row, an
unbalanced or reused transaction or a closed period. Changes and deletes of mutable
rows are audited, as described above.

Because the audit log gets one line per append request, a writer that
produces many rows should send them in batches of up to 1,000 rows rather
than one request per row.

When an owner is renamed, its rows move with it. When an owner is deleted,
its rows are deleted too. A soft-deleted owner keeps its rows until it is
purged, so a restore brings them back. For a ledger, deleting an owner
removes one side of its transactions, so archive an account that has
postings instead of deleting it.

On PostgreSQL and SQLite the rows move and go in the same transaction as
the rename or delete. If that fails, the rename or delete fails too, so a
new entity that later gets the same id never sees an old entity's rows.
On the file backend the rows move and go just after the rename or delete,
and a failure there is only logged.

On PostgreSQL and SQLite an append checks the owner and its lock date in
the same transaction that stores the rows, under the lock that entity
writes take. A posting that is in flight while someone closes the period
either lands before the period closes, or is checked against the new lock
date. On the file backend the two steps are not locked together, so a
posting that is in flight while someone closes the period can still land
in it.

## HTTP API

| Request | Purpose |
| --- | --- |
| `GET /api/v1/_records` | The declared record types. |
| `GET /api/v1/_records/{type}?owner=ID&from=&to=&after=&limit=` | One page of an owner's rows, oldest first. `after` is the cursor from the previous page. |
| `POST /api/v1/_records/{type}` | Append rows: `{"rows": [{"owner": "MTR-1", "ts": "2026-01-05T10:00:00Z", "fields": {"kwh": "1.25"}}]}`. |
| `GET /api/v1/_records/{type}/_aggregate?owner=ID&field=kwh&bucket=day` | Count, sum, minimum, maximum, average and latest value per time bucket. `bucket` is `hour`, `day`, `week` or `month`. Repeat `owner` to fold several owners, or leave it out to fold every owner you can read. `per_owner=true` keeps owners apart. |
| `GET /api/v1/_records/{type}/_transaction?group=KEY` | The rows of one ledger transaction. |
| `POST /api/v1/_records/{type}/_reverse` | Reverse a transaction: `{"group": "KEY"}`. |
| `PATCH /api/v1/_records/{type}/{id}` | Change a row of a mutable type: `{"revision": 0, "ts": "2026-01-05T00:00:00Z", "fields": {"hours": "7.5"}}`. `revision` is required; `ts` is optional. Answers the changed row. |
| `DELETE /api/v1/_records/{type}/{id}?revision=N` | Delete a row of a mutable type. Answers 204. |

Send decimals as strings, such as `"1.25"`, so no precision is lost.

A change or delete with a stale `revision` answers 409 with the code
`stale_revision`. A row whose owner you cannot read answers 404, the same as
a row that does not exist.

An aggregate folds at most 1,000,000 rows. Narrow `from`, `to` or `owner`
when a query needs more.

## MCP, scripts and the command line

The same service backs every surface, so the access rules are the same.

| Surface | Append | Read |
| --- | --- | --- |
| MCP | `append_records` | `list_record_types`, `list_records`, `aggregate_records` |
| Lua | `rela.append_records` | `rela.list_records`, `rela.aggregate_records` |
| CLI | `rela records append` | `rela records list`, `rela records aggregate` |

`rela records expire` drops rows past retention on demand. Only the command
line runs it, besides the hourly job. See [MCP server](mcp-server.md#record-tools),
[Lua scripting](lua-scripting.md#read-records-relalist_records-and-relaaggregate_records)
and [the CLI reference](cli-reference.md#rela-records).

## Roll-ups over records

An owner type can show a value computed from its rows as a property: the
`latest` value of a field, or its `sum` or `avg` over a `window`. See
[roll-ups over records](metamodel.md#roll-up-properties). The value is
computed on read with the reader's access, one record read per page.

## Trend charts

A trend such as "control coverage per quarter" is a value computed over the
graph at a point in time. rela does not store such values on its own. To
chart one over time, capture it as a record:

1. Declare an owner type with one entity per series, such as a `metric`
   entity `MET-control-coverage`.
2. Declare a record type on it, such as `measurement` with a decimal
   `value` field.
3. Add a [scheduled task](scheduled-tasks.md#capturing-a-metric-for-trend-charts)
   with `run_as:` that computes the value and appends one row with
   `rela.append_records`.
4. Add a [line card](data-entry.md#line-cards-display-line) to a dashboard.

**The value is computed under `run_as`, but anyone who can read the metric
entity can read it.** A record follows its owner, not the data that went
into it. A count over controls that only an auditor can read becomes, once
stored, readable by everyone who can read the metric. So give the metric
entity a read rule no wider than what the `run_as` identity can see. This is
the same rule as for elevated documents and digests: decide who may see the
result before you fold the data into it. A role with `read: ["*"]` reads the
metric too. Past rows stay readable to every reader of the metric, even after
you narrow access to the source data.

A capture task appends through the same check as `POST /api/v1/_records/{type}`:
its `run_as` identity needs `update` on the metric entity. `create` on the
owner type is not needed.

## Storage

Each backend stores records in its own way:

| Backend | Storage |
| --- | --- |
| Files | JSON Lines files under `records/`, one file per owner per month. |
| SQLite | The `records` table in `rela.db`. |
| PostgreSQL | The `records` table in the tenant's schema. |

Rows past their retention are dropped by a background job that runs every
hour. Set `RELA_RECORD_RETENTION_INTERVAL` to change the interval.

The record store is a separate service behind one interface, so a
specialised time-series backend can replace it later.

Records are part of the exports: `rela export --all` lists them under
`records`, and `rela db dump --data` and `rela db load --data` carry them as
`records/` files.

When you remove a record type from the schema, its rows are kept with
their owner until the owner is deleted. They are still in every export and
copied as stored, but the retention job does not expire them, and no
command deletes them by type. To bring them under a retention again,
declare the type again with a `retention`.
