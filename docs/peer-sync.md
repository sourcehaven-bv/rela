# Pulling from a sync peer

One rela can pull entities and relations from another rela. You declare the
other rela, the source, in `peers.yaml` in the project root. Each pull
compares this project with the source, fetches the rows that are new or
changed there, and writes them here through the normal write path.

A pull is one way. It reads the source and never writes to it. The source
decides what the pull may read: it authenticates the pull with a peer key and
applies its own `acl.yaml` to the peer, as for any other principal. See
[ACL security](acl-security.md#peer-keys) for peer keys.

## Set up a peer

On the source, create a peer key and grant the peer a read role:

```bash
(umask 077 && rela peer key create catalog-mirror > catalog-mirror.key)
```

Add the `peer_key.catalog-mirror` line the command prints to the source's
`.rela/secrets.yaml`, and give `peer:catalog-mirror` a role in the
source's `acl.yaml`.

On the pulling project, store the key in `.rela/secrets.yaml`:

```yaml
peer_credential.catalog: relapeer_...
```

Then declare the peer in `peers.yaml`:

```yaml
peers:
  catalog:
    description: The product catalog
    url: https://catalog.example.com
    credential: peer_credential.catalog
    every: 1h
    on_gone: detach
    entities:
      requirement:
        type: requirement
        ref: catalog_ref
        fields:
          title: title
          status: state
          content: content
      component:
        ref: catalog_ref
        fields:
          name: name
    relations:
      satisfies:
        fields:
          weight: weight
```

The schema needs an `external_ref` property on every mapped type, with a
system that only this peer uses:

```yaml
catalog_ref: {type: external_ref, system: catalog}
```

Check the file with `rela validate`. Pull once with `rela peer pull
catalog`, or let the scheduler pull it every hour.

## The keys

| Key | Meaning |
| --- | --- |
| `url` | The source's base address. Plain `http` works only for a loopback host unless `allow_insecure_http: true`. |
| `credential` | The global secret that holds the peer key. It must start with `peer_credential.`. |
| `every` | The schedule, as in `schedules.yaml`. At least `5m`. Without it the peer is pulled only by `rela peer pull`. |
| `on_gone` | `detach` (the default) or `delete`. See [Rows the source drops](#rows-the-source-drops). |
| `max_gone` | How many linked rows one pull may detach or delete. Default 10. With `on_gone: delete` it also caps the deletes of the last 24 hours. |
| `max_rows` | The most rows in scope on either side. Default 100,000. |
| `max_fetch` | The most rows one pull fetches. Default 10,000. The rest wait for the next pull. |
| `max_bytes` | The most bytes one pull reads in fetched rows. Default 256 MiB. The rest wait for the next pull. |
| `entities` | Source entity types, each mapped to a local type. |
| `relations` | Source relation types, each mapped to a local type that connects two mapped local types. |

Under `entities`, `type` is the local type (default: the source type's name),
`ref` is the local `external_ref` property that holds the source id, and
`fields` maps a local field to the source field it comes from. `content` is
the body. Under `relations`, `type` and `fields` work the same way; a
relation has no `ref`.

A mapped local type must not have faces and must not take its id from a
property. A source row with a face is not pulled.

## How a pull writes

- **The local id is always new.** A pulled row gets an id from the local
  type's prefix. The source id is kept in the `ref` property only. The source
  cannot choose a local id.
- **The source owns the mapped fields; the rest is local.** Fields not in
  `fields` are never written by a pull, so you can add local fields to a
  pulled row.
- **A change on both sides is a conflict.** The pull remembers, per field,
  the value both sides last agreed on. When only the source changed a field,
  the pull writes it. When only you changed it, the pull keeps yours. When
  both changed it, the pull leaves the row as it is and reports a conflict.
  It compares the row again on the next pull.
- **Relations follow the source.** The source wins on mapped relation
  fields. A relation is pulled once both of its ends are pulled. Its fields
  are checked against the local relation type first.
- **A row that stops waits its turn.** A row with a conflict or an error is
  fetched after the other changed rows on the next pull, so a few stuck rows
  cannot use up `max_fetch`. The report counts the rows still waiting.
- **A row you delete stays deleted.** If you delete a pulled entity or
  relation, later pulls do not recreate it.
- **A row the local schema refuses is reported.** A value that does not fit
  the local type, such as an unknown enum value, rejects the row. The pull
  tries it again when the source changes it.
- **A hidden field is not a cleared field.** When the source redacts a field
  for the peer, the pull leaves the local value alone.
- **Writes are attributed to the peer.** Every write runs as
  `system:peer:<name>`, through the same validation, automations and audit
  log as any other write. Give that identity a role in this project's
  `acl.yaml` that lets it create and update the mapped types, and delete
  them if `on_gone: delete`.

## Rows the source drops

When the source no longer lists a row the pull linked, the row is gone.
That happens when the source deletes it, or when the source's `acl.yaml`
stops letting the peer read it.

- `on_gone: detach` keeps the local row and stops pulling it. If the source
  lists it again, the pull links it again.
- `on_gone: delete` deletes a gone relation. It deletes a gone entity only
  when no mapped field was edited locally and the entity has no local
  relations. Otherwise it keeps and detaches the entity and reports why.
  Fields that are not mapped are deleted with the entity. A row that was
  there before the pull linked it, through its `ref`, is never deleted; it
  is detached.

A pull refuses to touch any gone row when more than `max_gone` linked rows
are gone, or when a type the pull linked rows of has no rows at the source
any more. Both look like a source that lost access, not like a source that
deleted rows. With `on_gone: delete` the pull also counts the deletes of
the last 24 hours against `max_gone`, so a source cannot delete in bulk a
few rows at a time. Check the source, then confirm with `rela peer pull
<name> --allow-gone N`, which allows up to `N` gone rows for that one pull.
A refused pull fails, so the scheduler reports it and tries again later.

## Pull by hand

```bash
rela peer pull catalog --dry-run
rela peer pull catalog
rela peer pull catalog --allow-gone 25
rela peer pull catalog -o json
```

`--dry-run` reports what the pull would do and writes nothing. It may
report a delete that a real pull turns into a detach, because the entity has
local relations. A pull by
hand takes the same lock as the scheduled pull, so the two never run at
once. A pull by hand fails with "a pull of this peer is already running"
while the scheduled pull runs, and the scheduler skips a pull while one by
hand runs. A dry run takes no lock.

## State

The pull keeps a ledger per peer in rela's state store: per source row, the
version it last applied, the local row, and the agreed value of each mapped
field as a digest. The ledger is not part of the graph. A change to the
mapping makes the next pull compare every row again.

## Security

The source is untrusted input. The pull:

- checks every row against the local schema before writing it, through the
  same entity manager as any other write;
- caps the rows it compares (`max_rows`), the rows it fetches (`max_fetch`),
  the bytes it fetches (`max_bytes`) and the size of each answer;
- checks that each fetched row has the id and type it asked for;
- never lets the source pick a local id or a local type;
- reads the credential only from the project's secrets (`.rela/secrets.yaml`,
  the systemd credential, or the desktop keychain), never from a flag, and
  never logs it;
- refuses plain http to a host that is not loopback, and never follows a
  redirect.

See [ACL security](acl-security.md#peer-pulls-peersyaml) for the threat
model.
