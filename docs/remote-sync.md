# Pulling from a remote

One rela can pull entities and relations from another rela. You declare the
other rela, the source, in `remotes.yaml` in the project root. Each pull
compares this project with the source, fetches the rows that are new or
changed there, and writes them here through the normal write path.

A pull is one way. It reads the source and never writes to it.

rela issues no credentials of its own. To the source, the pulling instance
is an ordinary principal. It authenticates where people do, at the source's
identity edge: with a machine-client token from the OIDC proxy in front of
the source (Pratique offers these), or through the trusted proxy that sets
the principal header. The source's `acl.yaml` then decides what that
principal may read, and `visible:` redacts its fields, as for anyone else.

## Set up a remote

On the source, give the pulling instance an identity at the identity edge,
such as a machine client in the OIDC proxy, and give that principal a read
role in the source's `acl.yaml`.

On the pulling project, store the token the identity edge issued in
`.rela/secrets.yaml`, under a name that starts with `remote_credential.`:

```yaml
remote_credential.catalog: eyJhbGciOi...
```

No Lua script can read a `remote_credential.` secret, and no other config
file may name one.

Then declare the remote in `remotes.yaml`:

```yaml
remotes:
  catalog:
    description: The product catalog
    url: https://catalog.example.com
    credential:
      secret: remote_credential.catalog
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
system that only this remote uses:

```yaml
catalog_ref: {type: external_ref, system: catalog}
```

Check the file with `rela validate`. Pull once with `rela remote pull
catalog`, or let the scheduler pull it every hour.

## The keys

| Key | Meaning |
| --- | --- |
| `url` | The source's base address. Plain `http` works only for a loopback host unless `allow_insecure_http: true`. |
| `credential` | Where the token comes from: exactly one of `env` (an environment variable whose name starts with `REMOTE_CREDENTIAL_`), `file` (an absolute path to a file of at most 16 KiB whose name starts with `remote_credential.`), or `secret` (a global secret whose name starts with `remote_credential.`). The token is never written in `remotes.yaml`. |
| `auth_header` | The request header that carries the token. Default `Authorization`, which sends it as a bearer token. |
| `run_as` | The identity a scheduled pull runs and writes as. Default `system:scheduler`, as for a scheduled task. A pull by hand runs as you. |
| `every` | The schedule, as in `schedules.yaml`. At least `5m`. Without it the remote is pulled only by `rela remote pull`. |
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
  for the remote, the pull leaves the local value alone.
- **A pull writes as whoever runs it.** A pull by hand runs as you (or
  `--run-as`); a scheduled pull runs as the remote's `run_as`, or as
  `system:scheduler`. Every write goes through the same validation,
  automations and audit log as any other write by that principal. Give it a
  role in this project's `acl.yaml` that lets it create and update the
  mapped types, and delete them if `on_gone: delete`. The audit log names
  that principal, with the triggered-by label `remote:<name>`; the `ref`
  property names the source row.
- **Pulled rows take this project's read rules.** The source's `acl.yaml`
  does not travel with the rows. Once pulled, a row is read under the
  local type's rules in this project's `acl.yaml`. Give each mapped type
  read rules at least as strict as the source's rules for the rows the remote
  can read, or people here can read what they could not read at the source.

## Rows the source drops

When the source no longer lists a row the pull linked, the row is gone.
That happens when the source deletes it, or when the source stops letting
this instance's principal read it: a narrowed `acl.yaml`, a revoked role, or
a token that expired or was revoked.

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
few rows at a time. Check the source, then confirm with `rela remote pull
<name> --allow-gone N`, which allows up to `N` gone rows for that one pull.
A refused pull fails, so the scheduler reports it and tries again later.

## Pull by hand

```bash
rela remote pull catalog --dry-run
rela remote pull catalog
rela remote pull catalog --allow-gone 25
rela remote pull catalog -o json
```

`--dry-run` reports what the pull would do and writes nothing. It may
report a delete that a real pull turns into a detach, because the entity has
local relations. A pull by hand takes the same lock as the scheduled pull, so the two never run at
once. A pull by hand fails with "a pull of this remote is already running"
while the scheduled pull runs, and the scheduler skips a pull while one by
hand runs. A dry run takes no lock.

## State

The pull keeps a ledger per remote in rela's state store: per source row, the
version it last applied, the local row, and the agreed value of each mapped
field as a digest. The ledger is not part of the graph and is never
exported. It is removed with the rest of the project's state. A row you
delete keeps only its source address and local id in the ledger, so the
pull does not recreate it. A change to the mapping makes the next pull
compare every row again.

## Security

The source is untrusted input. The pull:

- checks every row against the local schema before writing it, through the
  same entity manager as any other write;
- caps the rows it compares (`max_rows`), the rows it fetches (`max_fetch`),
  the bytes it fetches (`max_bytes`) and the size of each answer;
- checks that each fetched row has the id and type it asked for;
- never lets the source pick a local id or a local type;
- reads the token only from the source `credential` names: a
  `REMOTE_CREDENTIAL_` environment variable, a `remote_credential.` file,
  or a `remote_credential.` secret (`.rela/secrets.yaml`, the systemd
  credential, or the desktop keychain). The name marks the value as one
  that may be sent to a remote, so `remotes.yaml` cannot send another host
  secret, such as `RELA_DATABASE_URL`, to the URL it names. It never takes
  the token from a flag and never logs it;
- fails when the source answers 401 or 403, and refuses to detach or delete
  when a type it linked rows of comes back empty, so a lost identity never
  reads as rows that are gone;
- refuses plain http to a host that is not loopback, and never follows a
  redirect.

The pull does not carry the source's read rules. Pulled rows are read under
this project's `acl.yaml`, so set read rules on the mapped types that are at
least as strict as the source's.

See [ACL security](acl-security.md#remote-pulls-remotesyaml) for the threat
model.
