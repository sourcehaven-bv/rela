<!-- This file is auto-generated from docs-project/entities/. Do not edit directly. -->

# Records (time series and ledgers)

A record is a typed row with a timestamp, owned by one entity. Records suit
data that does not fit the graph: meter readings, measurements, events and
ledger postings. A meter can hold thousands of readings without each one
becoming an entity with relations.

Records are append-only. You cannot edit or delete a single row. A ledger
corrects a posting by appending its reversal.

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

A record type cannot share its key with an entity type. The field names `id`,
`owner`, `ts`, `recorded_at`, `recorded_by`, `group` and `reverses` are reserved.

## Ledgers

A ledger type groups rows into transactions by the `group_by` field. Each
transaction must balance: its `amount` values add up to zero, per
`commodity` when one is named. All rows of a transaction have the same `ts`.
rela refuses a transaction that does not balance, and refuses a second
transaction with a group key that is already used.

`closed_before` names a date property on the owner. A posting dated before
that date is refused, so a closed period stays closed.

To correct a transaction, reverse it. A reversal appends the same rows with
the amounts negated. Each transaction can be reversed once.

## Access control

Records have no grants of their own. They follow their owner:

- You can read an owner's rows when you can read the owner. To anyone else
  the owner's rows do not exist, and a request for them answers 404.
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

Record writes are not in the audit log. Each row stores who appended it
(`recorded_by`) and when (`recorded_at`).

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

Send decimals as strings, such as `"1.25"`, so no precision is lost.

An aggregate folds at most 1,000,000 rows. Narrow `from`, `to` or `owner`
when a query needs more.

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
