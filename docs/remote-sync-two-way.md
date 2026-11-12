<!-- This file is auto-generated from docs-project/entities/. Do not edit directly. -->

# Two-way sync with a remote

A remote with `push: true` in `remotes.yaml` is two-way. Each run first
pulls, as [Syncing with a remote](remote-sync.md) describes, and then sends
this project's changes to the remote. This guide covers what a push sends,
what happens when both sides change a field, and how a person settles what
the run could not.

## Pushes

One side drives: the project that declares the remote runs both
directions. The remote needs no `remotes.yaml` and keeps no state. After the
pull, the run pushes:

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
  422), the row goes on the conflict list as `refused`. It is not sent again
  until a local value it tried to send changes, or someone resolves it.
- **A create is not sent twice blindly.** When a create fails without
  saying whether the remote made the row, the row goes on the conflict list
  as `uncertain`. See [Uncertain creates](#uncertain-creates).
- **Rows mirrored from elsewhere are pushed too.** A row this project pulled
  from another remote, under a different `ref`, has no `ref` for this
  remote, so it is created here as well. Use `share` to leave such rows out.
- **No echo.** A value the run pushed becomes the agreed value, so the pull
  that sees it come back writes nothing. A run with no new edits on either
  side changes no version and writes no audit row on either side.
- **Deletes are sent only with `send_deletes`.** See [Deletes](#deletes).
- **Relations are pushed too.** A local relation of a mapped type is
  created on the remote when both of its ends are linked rows and you may
  read them. Its mapped meta flows both ways under the conflict rules, and
  every update carries `If-Match`, as for an entity. Its body comes from
  the remote only. When both sides made the same relation, the two are
  linked and their meta is merged. A relation type whose meta has a
  `visible:` grant in this project's `acl.yaml` is not pushed; the run
  reports it once. A refused relation push is held on the conflict list.

## Deletes

Two settings decide what a delete does, and both are off by default:

- `on_gone: delete` follows a delete on the remote: the run deletes the
  linked row here. See [Rows the source
  drops](GUIDE-remote-sync.md#rows-the-source-drops).
- `send_deletes: true` sends a delete made here: the run deletes the linked
  row on the remote. It needs `push: true`.

A delete never overwrites an edit blindly. Before it deletes a row on the
remote, the run fetches it. When a mapped field changed on the remote
since the last sync, the conflict rule decides: `hold` and `latest` put
the row on the conflict list as `local_deleted`, `remote` restores the row
here, and `local` deletes it anyway. (`latest` holds, because neither side
records when a row was deleted.) Otherwise the run sends `DELETE` with
`If-Match`, so a remote edit that lands between the fetch and the delete
is a 412, and the run fetches again. When the remote refuses the delete
(400, 403, 409 or 422), the row is held as `local_deleted` with cause
`refused`. Relations are deleted before the entities they connect.

The same holds the other way. When the remote deletes a row that was
edited here since the last sync, `on_gone: delete` applies the rule: `hold`
and `latest` keep the row and hold it as `remote_deleted`, `local` keeps it
and creates it on the remote again, and `remote` deletes it here.

Deletes in both directions count against `max_gone`. A run that would
delete more, or would empty a type on either side, holds every delete and
detach until you confirm with `--allow-gone`; the rest of the run applies.

The remote applies its own rules to a delete, as for any caller of its
API: its ACL, and its cascade or soft delete. Give the push principal the
right to delete the mapped types on the remote.

### Choosing what to share

`share` selects the local rows a push sends. It is a condition in the
predicate language on the local row as the pusher reads it, and reads the
row only; `related(...)` is refused. A linked row that stops meeting it is
no longer pushed. Without `share`, every row of the type is pushed. Mind
that the first run with `push: true` creates on the remote every local row
of the mapped types that meets `share` and has no `ref`.

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

## Approval is a workflow, not sync

Sync has no approval step, and none should be added to it. A run sends what
`share` and `fields` select, and the remote decides what it accepts. To have
a person approve what the remote receives, model the approval in your own
workflow:

1. Add a local field that the approval sets, such as `approved_for_portal`,
   and guard it with a transition or a `fields:` grant in `acl.yaml`, so
   only an approver can set it.
2. Share only rows that have it: `share: "entity.approved_for_portal == true"`.

The approval is then an ordinary write, with its own audit record, and it
works the same whether the remote is rela or not. The same holds the other
way: the remote's own workflow decides what it lets this project's
principal read.

## Conflicts

A field conflicts when both sides changed it since the last sync. The
remote's `conflict` rule decides what happens, and a type's `conflict` or a
field's entry in `field_conflicts` overrides it:

| Rule | What happens |
| --- | --- |
| `hold` | The field is written on neither side and goes on the conflict list. The default. |
| `latest` | The side that changed the row last wins. The other side's value is kept on the conflict list. |
| `local` | This project's value wins. On a remote with `push: true` it is sent to the remote. |
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

### Clock bounds for `latest`

`latest` compares the times the two rows last changed. There is one time
per row, not per field, so an edit to another field of the row counts too.
The remote serves its time as `_updated` only when nothing on the row is
hidden from this project's principal, and this project uses its own time
under the same condition. `latest` holds the field instead, and says why
in the entry's `fallback`, when:

- a side served no time (`time_withheld`);
- a time lies more than `max_clock_skew` in the future, lies more than
  `max_clock_skew` before the row's last sync, or does not parse
  (`time_untrusted`);
- both times are equal (`same_time`).

`max_clock_skew` defaults to `5m` and may be at most `1h`.

The remote's time comes from the remote's clock. A remote that sets its
clock wrong, or lies about it, wins every conflict it can within those
bounds. For a remote you do not trust, use `hold` or `local`. On the
file-system backend, the time of a row nobody wrote since the server
started is the file's modification time, so a checkout or a copy moves it.

### Retention

When `latest` decides a field, the entry stays on the conflict list as
`superseded` and keeps the value that lost (up to 16 KiB), so you can put
it back by hand. It blocks nothing.

`superseded_retention` sets how long it stays, per remote. The default is
30 days. The setting must be a duration of 0 or more, such as `30d` or
`12h`; any other value stops the remote from loading.

A value of `0` keeps no losing local value at all. The entry keeps only the
digests of the two values, and stays until it is dismissed or its row is no
longer linked. The CLI and the Remotes page say that the overwritten value
was not kept. A losing remote value is kept either way, because the remote
still has it.

## The conflict list

The conflict list holds what waits for a person. Each entry names the local
row and, for most reasons, one field. The entry's reason says why it is
there:

| Reason | Meaning | What you can do |
| --- | --- | --- |
| `both_changed` | Both sides changed the field and the rule held it. | Take the local or the remote value. |
| `refused` | The remote refused a push. `cause` and `status` say how. | Take local to send it again, or take remote to stop sending. |
| `uncertain` | A create failed without saying whether the remote made the row. | Send it again, or link it to the remote row. |
| `superseded` | `latest` decided the field; the losing value is kept. | Dismiss it once you have checked it. |
| `local_deleted` | The row was deleted here, and the remote changed or refused it. | Take local to delete it on the remote, or take remote to restore it here. |
| `remote_deleted` | The remote deleted the row, and it was edited here. | Take local to create it on the remote again, or take remote to delete it here. |

A refused or uncertain entry keeps a structured `cause` (such as
`refused`, `server_error`, `timeout` or `no_answer`) and the HTTP status,
not the remote's error text. A `latest` entry that held keeps its
`fallback`.

Each entry keeps the remote's value of the field (up to 16 KiB). A
`superseded` entry also keeps the local value that lost, under
`superseded_retention`. The list is operator state in rela's state store,
outside the graph.

An entry goes away when both sides agree again, when it is resolved, linked
or dismissed, or when its row is no longer linked. A `superseded` entry also
goes when `rela history-purge` purges its entity, or at the first sync after
it is older than `superseded_retention`.

### Acting on an entry

You can act on the list from the CLI or from the Remotes page of the
data-entry app (`/remotes/<name>`).

```bash
rela remote conflicts portal
rela remote conflicts portal -o json
rela remote resolve portal TKT-ABC123 title --take local
rela remote resolve portal TKT-ABC123 --take remote
rela remote resolve portal TKT-ABC123 --dismiss
rela remote link portal TKT-ABC123 ticket/T-981
```

| Action | CLI | Remotes page |
| --- | --- | --- |
| Take the local value | `rela remote resolve <name> <local> [field] --take local` | Keep local |
| Take the remote value | `rela remote resolve <name> <local> [field] --take remote` | Take remote |
| Dismiss a `superseded` entry | `rela remote resolve <name> <local> [field] --dismiss` | Dismiss |
| Link an uncertain create | `rela remote link <name> <local> <address>` | Link |

Resolving takes the field's value from one side for the next sync to send
to the other. It writes neither side itself: it marks the losing value as
already agreed, and the next sync writes the winning value through the
normal write path on each side. A refused push resolved to `local` is sent
again; resolved to `remote`, the values it tried to send stop counting as
changes, and the remote's values are pulled. A refused or uncertain create
has no remote value, so it resolves to `local` only.

- The CLI resolves every entry of the row when you leave out the field, and
  then runs one sync, as you.
- The Remotes page acts on one entry at a time and does not sync. The next
  scheduled pull applies the decision; the page shows when that is.

For a relation, name the row by the local relation key that `rela remote
conflicts` shows, such as `REQ-1--satisfies--CMP-2`.

`--dismiss` removes the `superseded` entries of the row, or of one field,
and changes no data. It refuses a held entry: resolve that with `--take`.

On the Remotes page every action needs `remote:sync` and read access to the
entry's row and field. Taking the remote value also needs the right to edit
the field, because the next pull writes it here. Linking needs the right to
edit every field the create sent, because the next sync merges the remote
row into the local one. Taking the remote side of a `remote_deleted` entry
needs the right to delete the row. A `local_deleted` entry names a row that
no longer exists here, so the Remotes page does not show it; resolve it
with the CLI. A button you may not use is
not shown. Each action is written to the audit log as `remote-resolve`,
`remote-dismiss` or `remote-link`, with the row and without values.

### Uncertain creates

A create is `uncertain` when the run cannot tell whether the remote made the
row. That covers a broken connection, a cancelled run, a timeout, a 429, 503
or other 5xx, and a success answer that does not name the new row. Only a
refused credential or a connection that was never made is retried on the
next run without a person.

Check the remote first. Then pick one of two ways out:

- **The remote has no such row.** Send it again: resolve the entry to
  `local`. The next sync creates the row.
- **The remote has the row.** Link the two: `rela remote link <name>
  <local> <type>/<id>`, or Link on the Remotes page, with the remote row's
  address. The next sync compares the two rows field by field and sets the
  local `ref`. Linking writes neither side. It refuses an address of
  another type than the one the local type maps from, and an address or a
  local row that is already linked. If the remote does not list the
  address, the next sync detaches the row.

Do not send again when the remote has the row. The remote would then hold
two copies, and the next pull would bring the second one in as a new local
row. If that happened, delete one of the copies.

## Security

A push:

- writes only the mapped fields, of rows that meet `share`, through the
  remote's normal write API as the remote's own principal, so the remote's
  `acl.yaml` bounds it as it bounds any client;
- never overwrites a remote change: each update carries `If-Match`, and a
  412 is merged first;
- trusts the remote's clock only within `max_clock_skew` of now and of
  the last sync, and holds a field when it cannot tell which side changed
  it last;
- pushes no relation of a type whose meta this project hides with
  `visible:`;
- keeps a losing local value for at most `superseded_retention` (default
  30 days, `0` for none), and drops it when the entity's history is purged;
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

See [ACL security](acl-security.md#pushes-push-true) for the threat model.
