# Syncing with a remote

One rela can pull entities and relations from another rela. You declare the
other rela, the source, in `remotes.yaml` in the project root. Each pull
compares this project with the source, fetches the rows that are new or
changed there, and writes them here through the normal write path.

A pull is one way. It reads the source and never writes to it. A remote
with `push: true` is two-way: after each pull, the run also sends this
project's changes to the remote. See [Two-way sync](#two-way-sync).

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
| `url` | The source's base address. Plain `http` works only for a loopback host unless `allow_insecure_http: true`. That setting sends the credential in clear text, so use it only on a private network or with TLS ending at a local proxy; each pull then logs a warning. |
| `credential` | Where the token comes from: exactly one of `env` (an environment variable whose name starts with `REMOTE_CREDENTIAL_`), `file` (an absolute path to a regular file of at most 16 KiB whose name starts with `remote_credential.`; a symlink must lead to a file with such a name, and a file with more than one hard link is refused), or `secret` (a global secret whose name starts with `remote_credential.`). The token is never written in `remotes.yaml`. |
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
| `push` | `true` makes the remote two-way. See [Two-way sync](#two-way-sync). Default `false`. |
| `conflict` | The rule for a field both sides changed since the last sync: `hold` (the default), `latest`, `local` or `remote`. See [Conflicts](#conflicts). |
| `max_clock_skew` | How far in the future a last-updated time may lie before `latest` stops trusting it. Default `5m`. |

Under `entities`, `type` is the local type (default: the source type's name),
`ref` is the local `external_ref` property that holds the source id, and
`fields` maps a local field to the source field it comes from. `content` is
the body. Under `entities`, a remote with `push` also takes `share`, a
condition on the local row (see [Two-way sync](#two-way-sync)), and a
type's `conflict` and per-field `field_conflicts` override the remote's
rule. Under `relations`, `type` and `fields` work the same way; a
relation has no `ref`, and on a remote with `push` a relation type also
takes `conflict` and `field_conflicts`.

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
  both changed it, the pull leaves that field as it is on both sides and
  puts it on the remote's conflict list. The row's other fields still
  merge. The field stays on the list until both sides agree again or
  someone resolves it. See [Conflicts](#conflicts).
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

## Two-way sync

With `push: true`, one side drives: the project that declares the remote
runs both directions. The remote needs no `remotes.yaml` and keeps no
state. Each run first pulls, as above, and then pushes:

- **A changed field is sent.** A mapped field whose local value moved off
  the value both sides last agreed on is sent to the remote with a `PATCH`
  through the remote's normal write API. Fields held on the conflict list
  are not sent.
- **A new local row is created on the remote.** A row of a mapped type that
  has no `ref` and meets `share` is created with a `POST`. The remote picks
  its id, as for any other caller. This project keeps the local id and the
  remote id in its ledger, and sets the row's `ref` to the remote id. If the
  ledger is lost, the `ref` links the two rows again, so the row is not
  created twice. A row that came from the remote is never created there
  again.
- **The remote decides.** The push writes as the same principal the pull
  reads as, with the same credential. The remote's `acl.yaml`, validation,
  automations and audit log apply as for any other write by that principal.
  Give it a role on the remote that may create and update the mapped types.
- **A remote change is never overwritten.** Every update carries the
  version the run last saw (`If-Match`). When the remote row changed since,
  the remote answers 412; the run fetches the row, merges it as a pull
  does, and sends once more. If it changed again, the row waits for the
  next run.
- **A refusal is held.** When the remote refuses a push (400, 403, 409 or
  422),
  the row goes on the conflict list with the remote's reason. It is not
  sent again until a local value it tried to send changes, or someone
  resolves it.
- **A create is not sent twice blindly.** When a create fails without
  saying whether the remote made the row, the row goes on the conflict list
  as `uncertain`. That covers a broken connection, a cancelled run, a 429,
  503 or other 5xx, and a success answer that does not name the new row.
  Only a refused credential or a connection that was never made is retried
  on the next run. Check the remote; if
  the row is there, the next pull brings it in as a new local row, so
  delete one of the two copies. Then resolve the entry to `local` to send
  the row again, or leave it.
- **Rows mirrored from elsewhere are pushed too.** A row this project pulled
  from another remote, under a different `ref`, has no `ref` for this
  remote, so it is created here as well. Use `share` to leave such rows out.
- **No echo.** A value the run pushed becomes the agreed value, so the pull
  that sees it come back writes nothing. A run with no new edits on either
  side changes no version and writes no audit row on either side.
- **Deletes are not sent.** Deleting a row on one side does not delete it
  on the other. A row this project created on the remote is never deleted
  here by `on_gone: delete`; when the remote stops listing it, it is
  detached.
- **Relations are pushed too.** A local relation of a mapped type is
  created on the remote when both of its ends are linked rows and you may
  read them. Its mapped meta flows both ways under the conflict rules, and
  every update carries `If-Match`, as for an entity. Its body comes from
  the remote only. When both sides made the same relation, the two are
  linked and their meta is merged. A relation type whose meta has a
  `visible:` grant in this project's `acl.yaml` is not pushed; the run
  reports it once. A refused relation push is held on the conflict list.
  Deleting a relation is not sent.

`share` selects the local rows a push sends. It is a condition in the
predicate language on the local row as the pusher reads it, and reads the
row only; `related(...)` is refused. A linked row that stops meeting it is no longer pushed. Without
`share`, every row of the type is pushed. Mind that the first run with
`push: true` creates on the remote every local row of the mapped types that
meets `share` and has no `ref`.

```yaml
remotes:
  portal:
    url: https://portal.example.com
    credential: {secret: remote_credential.portal}
    every: 15m
    push: true
    entities:
      ticket:
        ref: portal_ref
        share: "entity.customer == 'acme'"
        fields: {title: title, status: status, content: content}
```

Approval is not part of sync. To have a person approve what the remote
receives, map a local field that your workflow sets on approval, and share
only rows that have it.

## Conflicts

A field conflicts when both sides changed it since the last sync. The
remote's `conflict` rule decides what happens, and a type's `conflict` or a
field's entry in `field_conflicts` overrides it:

| Rule | What happens |
| --- | --- |
| `hold` | The field is written on neither side and goes on the conflict list. The default. |
| `latest` | The side that changed the row last wins. The other side's value is kept on the conflict list. |
| `local` | This project's value wins and is sent to the remote. |
| `remote` | The remote's value wins and is written here. |

```yaml
remotes:
  portal:
    push: true
    conflict: latest
    entities:
      ticket:
        ref: portal_ref
        fields: {title: title, status: status}
        field_conflicts: {status: remote}
```

`latest` compares the times the two rows last changed. There is one time
per row, not per field, so an edit to another field of the row counts too.
The remote serves its time as `_updated` only when nothing on the row is
hidden from this project's principal, and this project uses its own time
under the same condition. `latest` holds the field instead, and says why
in the entry's `fallback`, when:

- a side served no time (`time_withheld`);
- a time lies more than `max_clock_skew` in the future, lies before the
  row's last sync, or does not parse (`time_untrusted`);
- both times are equal (`same_time`).

The remote's time comes from the remote's clock. A remote that sets its
clock wrong, or lies about it, wins every conflict it can within those
bounds. For a remote you do not trust, use `hold` or `local`. On the
file-system backend, the time of a row nobody wrote since the server
started is the file's modification time, so a checkout or a copy moves it.

When `latest` decides a field, the entry stays on the conflict list as
`superseded` and keeps the value that lost (up to 16 KiB), so you can put
it back by hand. It blocks nothing. Remove it with `--dismiss`.

The conflict list holds what waits for a person: a held field, a push the
remote refused, and the entries `latest` left.

```bash
rela remote conflicts portal
rela remote conflicts portal -o json
rela remote resolve portal TKT-ABC123 title --take local
rela remote resolve portal TKT-ABC123 --take remote
rela remote resolve portal TKT-ABC123 --dismiss
```

`rela remote resolve` takes the field's value from one side for the next
sync to send to the other. Without a field it resolves every entry of the
row. It writes neither side itself: it marks the losing value as already
agreed, then runs one sync, which writes the winning value through the
normal write path on each side, as you. A refused push resolved to `local`
is sent again; resolved to `remote`, the values it tried to send stop
counting as changes, and the remote's values are pulled. A refused or
uncertain create has no remote value, so it resolves to `local` only.
For a relation, name the row by the local relation key that `rela remote
conflicts` shows.

`--dismiss` removes the `superseded` entries of the row, or of one field,
and changes no data. It refuses a held entry: resolve that with `--take`.

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

On a remote with `push`, `rela remote pull` runs the whole sync, and the
dry run also reports what it would push.

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
exported. It is removed with the rest of the project's state. A change to
the mapping makes the next pull compare every row again.

On a remote with `push`, the ledger also maps each local id to its remote
id, and keeps the remote row's last version for `If-Match`. Only this
project writes the mapping.

The conflict list is a second document beside the ledger. Each entry names
the local row and field, and keeps the remote's value of the field (up to
16 KiB) or the remote's reason for a refusal. A `superseded` entry also
keeps the local value that lost, up to 16 KiB. It is operator state: it is
read with `rela remote conflicts`, not through `acl.yaml`. A held entry goes
when both sides agree again, when it is resolved, or when its row is no
longer linked. A `superseded` entry goes when it is dismissed or its row is
no longer linked.

The ledger keeps no copy of a row that is gone here:

- When you delete a pulled row, the next pull keeps only its source address
  and local id, so it does not recreate the row. The field digests go.
- The ledger holds no history, so purging history leaves nothing behind in
  it. The conflict list is not history either: purging history does not
  remove a value a `superseded` entry keeps. Dismiss the entry to remove it.
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

A push:

- writes only the mapped fields, of rows that meet `share`, through the
  remote's normal write API as the remote's own principal, so the remote's
  `acl.yaml` bounds it as it bounds any client;
- never overwrites a remote change: each update carries `If-Match`, and a
  412 is merged first;
- trusts the remote's clock only within `max_clock_skew` and not before
  the last sync, and holds a field when it cannot tell which side changed
  it last;
- pushes no relation of a type whose meta this project hides with
  `visible:`;
- never sends an id: the remote picks it;
- treats the remote's answer as untrusted: it checks that a create answers
  with a new row of the type it asked for, removes control characters from
  a refusal's reason and caps its size;
- fails the run when the remote answers 401, and stops it on 429 or 503.

What a push sends leaves this project's `acl.yaml`. The push reads the rows
of the mapped types as the person or job it runs as, like the pull: it
sends no row and no field that principal cannot read, and does not name a
hidden row in its report. Within that, `share` and `fields` decide what the
remote receives. A scheduled push with no `run_as` runs as
`system:scheduler`, so give that principal, or the remote's `run_as`, read
access to what the remote should get.

The pull does not carry the source's read rules. Pulled rows are read under
this project's `acl.yaml`, so set read rules on the mapped types that are at
least as strict as the source's.

See [ACL security](acl-security.md#remote-pulls-remotesyaml) for the threat
model.
