<!-- This file is auto-generated from docs-project/entities/. Do not edit directly. -->

# Syncing with a remote

One rela can pull entities and relations from another rela. You declare the
other rela, the source, in `remotes.yaml` in the project root. Each pull
compares this project with the source, fetches the rows that are new or
changed there, and writes them here through the normal write path.

A pull is one way. It reads the source and never writes to it. A remote
with `push: true` is two-way: after each pull, the run also sends this
project's changes to the remote. [Two-way sync](remote-sync-two-way.md)
explains pushes, conflict rules and the conflict list.

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
file may name one. In the same way, no other config file may name a
`REMOTE_CREDENTIAL_` environment variable: `ai.yaml` `api_key_env` and
`mail.yaml` `password_env` refuse it.

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
| `url` | The source's base address. Plain `http` works only for a loopback host unless `allow_insecure_http: true`. That setting sends the credential in clear text, so use it only on a private network or with TLS ending at a local proxy; each pull then logs a warning. |
| `credential` | Where the token comes from: exactly one of `env` (an environment variable whose name starts with `REMOTE_CREDENTIAL_`), `file` (an absolute path to a regular file of at most 16 KiB whose name starts with `remote_credential.`; a symlink must lead to a file with such a name, and a file with more than one hard link is refused), or `secret` (a global secret whose name starts with `remote_credential.`). The token is never written in `remotes.yaml`. |
| `auth_header` | The request header that carries the token. Default `Authorization`, which sends it as a bearer token. |
| `run_as` | The identity a scheduled pull runs and writes as. Default `system:scheduler`, as for a scheduled task. A pull by hand runs as you. |
| `every` | The schedule, as in `schedules.yaml`. At least `5m`. Without it the remote is pulled only by `rela remote pull`. |
| `on_gone` | `detach` (the default) or `delete`. See [Rows the source drops](#rows-the-source-drops). |
| `max_gone` | How many linked rows one run may detach or delete, in either direction. Default 10. It also caps the deletes of the last 24 hours. |
| `send_deletes` | `true` sends a delete made here to the remote. Needs `push: true`. Default `false`. See [Deletes](GUIDE-remote-sync-two-way.md#deletes). |
| `max_rows` | The most rows in scope on either side. Default 100,000. |
| `max_fetch` | The most rows one pull fetches. Default 10,000. The rest wait for the next pull. |
| `max_bytes` | The most bytes one pull reads in fetched rows. Default 256 MiB. The rest wait for the next pull. |
| `entities` | Source entity types, each mapped to a local type. |
| `relations` | Source relation types, each mapped to a local type that connects two mapped local types. |
| `push` | `true` makes the remote two-way. Default `false`. See [Two-way sync](remote-sync-two-way.md). |
| `conflict` | The rule for a field both sides changed since the last sync: `hold` (the default), `latest`, `local` or `remote`. See [Conflicts](remote-sync-two-way.md#conflicts). |
| `max_clock_skew` | How far in the future a last-updated time may lie before `latest` stops trusting it. Default `5m`, at most `1h`. |
| `superseded_retention` | How long a `superseded` entry keeps the local value that lost, such as `30d` or `12h`. Default `30d`. `0` keeps no losing local value. See [Retention](remote-sync-two-way.md#retention). |

Under `entities`, `type` is the local type (default: the source type's name),
`ref` is the local `external_ref` property that holds the source id, and
`fields` maps a local field to the source field it comes from. `content` is
the body. Under `entities`, a remote with `push` also takes `share`, a
condition on the local row, and a type's `conflict` and per-field
`field_conflicts` override the remote's rule. Under `relations`, `type` and
`fields` work the same way; a relation has no `ref`, and on a remote with
`push` a relation type also takes `conflict` and `field_conflicts`.

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
  both changed it, the remote's conflict rule decides. Under the default
  rule, `hold`, the pull leaves that field as it is on both sides and puts
  it on the remote's conflict list. The row's other fields still merge. See
  [Conflicts](remote-sync-two-way.md#conflicts).
- **Relations follow the source.** The source wins on mapped relation
  fields. A relation is pulled once both of its ends are pulled. Its fields
  are checked against the local relation type first.
- **A row that stops waits its turn.** A row with an error is
  fetched after the other changed rows on the next pull, so a few stuck rows
  cannot use up `max_fetch`. The report counts the rows still waiting.
- **A row the puller cannot read is not named.** If the pull finds a
  local row that the person it runs as cannot read, it does not write or
  delete it. The report shows a conflict, or a kept row, without the local
  id or type.
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
  Fields that are not mapped are deleted with the entity. On a remote
  without `push`, a row that was there before the pull linked it, through
  its `ref`, is never deleted; it is detached. On a remote with `push`, a
  row edited here since the last sync follows the conflict rule instead;
  see [Deletes](GUIDE-remote-sync-two-way.md#deletes).

A run holds every detach and delete when more than `max_gone` linked rows
are gone, or when a type the run linked rows of has no rows left on one
side. Both look like a side that lost access, not like a side that deleted
rows. The run also counts the deletes of the last 24 hours, in both
directions, against `max_gone`, so a side cannot delete in bulk a few rows
at a time. Only the detaches and deletes wait; the rest of the run applies.
Check both sides, then confirm with `rela remote pull <name> --allow-gone
N`, which allows up to `N` of them for that one run. A run that held them
fails, so the scheduler reports it and tries again later.

## Commands

| Command | What it does |
| --- | --- |
| `rela remote pull <name>` | Sync the remote now: pull, and push too on a remote with `push`. |
| `rela remote conflicts <name>` | List the remote's conflict list. |
| `rela remote resolve <name> <local> [field] --take local\|remote` | Resolve a held entry to one side, then sync. |
| `rela remote resolve <name> <local> [field] --dismiss` | Remove the `superseded` entries `latest` left. |
| `rela remote link <name> <local> <address>` | Link an uncertain create to the remote row it made. |

Every command takes `-o json`. [Two-way sync](remote-sync-two-way.md)
explains the conflict list and when to use each of the last three.

```bash
rela remote pull catalog --dry-run
rela remote pull catalog
rela remote pull catalog --allow-gone 25
rela remote pull catalog -o json
```

On a remote with `push`, `rela remote pull` runs the whole sync, and the
dry run also reports what it would push.

`--dry-run` reports what the pull would do and writes nothing. It may
report a delete that a real pull turns into a detach, because the entity has
local relations. A pull by hand takes the same lock as the scheduled pull, so
the two never run at once. A pull by hand fails with "a pull of this remote
is already running" while the scheduled pull runs, and the scheduler skips a
pull while one by hand runs. A dry run takes no lock.

## The Remotes page

The data-entry app shows every remote at `/remotes`, linked from the sidebar
footer. Each remote shows:

- its settings: URL, description, schedule, `run_as`, `push`, the conflict
  rule, `on_gone`, `max_clock_skew`, `superseded_retention` and the mapped
  types and fields with their rules;
- where its credential is read from (`env`, `file` or `secret`), never the
  credential's name or value;
- its run health: the last successful pull, the next scheduled pull, the
  next retry after a failure, the number of failures in a row, and whether a
  run holds the remote now;
- how many linked rows and conflict entries you may read, by reason.

`/remotes/<name>` lists the remote's conflict list and lets you act on each
entry. [Two-way sync](remote-sync-two-way.md#acting-on-an-entry) explains
the actions. The page never starts a sync. A decision made there is applied
by the next scheduled pull, which the page shows. To apply it at once, run
`rela remote pull <name>`.

### Who may use it

The page needs the built-in permission `remote:sync`. Grant it to a role by
name:

```yaml
# acl.yaml
roles:
  integrator:
    permissions: [remote:sync]
```

Without it the page answers 403, and the response names `remote:sync`.
Without an `acl.yaml` every principal holds it; with `--read-only` nobody
does.

The permission does not widen what you read. The page shows a conflict entry
only when you may read its local row and, for a field entry, the field.
Counts include only those entries. Taking the remote value of a field also
needs the right to edit that field, because the next pull writes it. See
[ACL security](acl-security.md#remote-sync-page-remotesync) for the threat
model.

The CLI commands read the conflict list as operator state, without
`acl.yaml`. Run them only where the operator may read every row.

## State

The pull keeps a ledger per remote in rela's state store: per source row, the
version it last applied, the local row, and the agreed value of each mapped
field as a digest. The ledger is not part of the graph and is never
exported. It is removed with the rest of the project's state. A change to
the mapping makes the next pull compare every row again.

On a remote with `push`, the ledger also maps each local id to its remote
id, and keeps the remote row's last version for `If-Match`. Only this
project writes the mapping.

The conflict list is a second document beside the ledger.
[Two-way sync](remote-sync-two-way.md#the-conflict-list) describes what it
keeps and how long.

The ledger keeps no copy of a row that is gone here:

- When you delete a pulled row, the next pull keeps only its source address
  and local id, so it does not recreate the row. The field digests go.
- The ledger holds no history, so purging history leaves nothing behind in
  it. A committed `rela history-purge` of an entity also drops its
  `superseded` entries, and those of relations it is an end of, from the
  conflict list of every remote. It does so on a backend without history
  too. If that step fails, the purge says so; run it again.
- When you remove a remote from `remotes.yaml`, its ledger and conflict list
  are deleted by the next pull of any remote and when the scheduler starts.

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
model, and [Two-way sync](remote-sync-two-way.md#security) for what a push
adds.
