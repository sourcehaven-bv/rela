<!-- This file is auto-generated from docs-project/entities/. Do not edit directly. -->

# Comments: Annotating Entities, Fields and Text

Comments let people annotate an entity without changing it: a remark on a
property, on a view section, or on a selected range of the markdown body.

They are deliberately **not** part of your graph. A comment is a remark *about*
an entity, not a fact *in* your domain model, so comments live in their own
store with their own permissions — outside `store.Store`, outside the audit log,
outside `/api/v1/_schema`. That separation is what makes the author
**unforgeable**: rela owns the read and write path end to end, writes the author
from the request principal, and never reads it from the request body.

## Enabling comments

Commenting is off until you ask for it. Add a top-level `comments:` block to
`schema.yaml`:

```yaml
comments:
  enabled: true
  on: ["ticket", "feature"]
```

`on:` lists the entity types that accept comments. The wildcard `"*"` means
every type your project declares:

```yaml
comments:
  enabled: true
  on: ["*"]
```

An empty `on:` with `enabled: true` is a **load error**, not a silent
"everything" or a silent "nothing" — both readings are defensible, so rela
refuses to guess.

With no `comments:` block at all the feature does not exist: the routes answer
404, no storage directory is created, and `/api/v1/_schema` is byte-identical to
a rela built before commenting existed.

### Where comments are stored

On the default (filesystem) backend, one YAML file per target under
`.rela/comments/`:

```text
.rela/comments/
  TKT-001.yaml
  TKT-001@draft.yaml
  FEAT-002.yaml
```

They are diffable and hand-editable like the entities they annotate. A target's
whole thread lives in one file, so writes are atomic (temp file, fsync, rename)
— a torn write would otherwise lose a whole conversation rather than one remark.

## Permissions

Six permissions, granted through a role's `permissions:` list exactly like the
`history:read` family:

| Permission | Grants |
|---|---|
| `comment:read` | See a target's comments |
| `comment:add` | Post a comment |
| `comment:update-own` | Edit or resolve your own comments |
| `comment:update-any` | Edit or resolve anyone's |
| `comment:delete-own` | Delete your own |
| `comment:delete-any` | Delete anyone's |

```yaml
roles:
  reviewer:
    read: ["ticket"]
    permissions:
      - comment:read
      - comment:add
      - comment:update-own
      - comment:delete-own

  editor:
    read: ["ticket"]
    permissions:
      - comment:read
      - comment:add
      - comment:update-any
      - comment:delete-any
```

`-any` implies `-own`. "Own" is a comparison of the stored author against the
requesting principal — rela has no `own` primitive in the graph, and comments
are not in the graph, so there is no edge to test.

Two rules are enforced and worth knowing:

- **`comment:read` is floored by the target's read verdict.** A principal who
  cannot read an entity cannot read its comments however the comment grants
  read, and cannot tell "no comments" from "not allowed" — the response is the
  same 404 a missing entity produces. Whether an entity exists stays a secret.
- **A mutating permission requires `comment:read`**, validated when `acl.yaml`
  loads. A role granting only `comment:delete-any` would let its holder remove
  comments it can never see. `comment:add` is exempt: write-only commenting is a
  coherent posture, the same way `create` is exempt from the entity write⊆read
  rule.

A permission conferred by an **ownership relation to the target** is honoured,
so "the assignee may comment on their own ticket" works without a special case.

### Identity is required

A comment is refused when the request has no resolvable identity, rather than
being stored as `unknown`. A comment nobody is recorded as having written could
never satisfy an `-own` check, so its author could neither edit nor delete it.

If you run rela-server without authentication, set `RELA_DATAENTRY_USER` or
point `-principal-header` at whatever your proxy injects.

## The three anchor kinds

### Property

Attaches to a named property. Drift-free by construction: a property name is a
*name*, not an offset, so it survives any edit to the entity.

In the UI, a small indicator sits beside each property label — a count when the
field has comments, a green check when they are all resolved, and a faint ⊕ on
hover when it has none.

### Section

Attaches to a view section by its operator-authored `sectionId` from
`data-entry.yaml`. Also a name, so also drift-free.

### Text range

Attaches to selected text in the markdown body. Select text, click **Comment**,
and the range renders highlighted.

This one anchors to content people can edit, so it works differently: rela
stores the quote plus its surrounding context (prefix, suffix, containing
sentence, nearest heading, paragraph index) and **re-locates it on every read**.
Byte offsets are never stored — an offset is invalidated by any edit earlier in
the body, and on the filesystem backend by a plain re-save, since rela reflows
markdown to 80 columns on write.

Consequences worth understanding:

- **A comment survives edits around it.** Insert a paragraph above the quoted
  text and the highlight still lands on the right words.
- **A comment on text that was edited away is reported detached**, never
  silently re-attached to something else. It still appears in the comments
  panel, flagged, showing what it was written about.
- **A match below the confidence threshold is flagged "may have moved"** rather
  than being presented as an exact location.

### Images and diagrams

An image or a mermaid/PlantUML diagram cannot be text-selected, so it gets its
own affordance: hover it and a comment button appears. Behind the scenes these
anchor to the block's markdown source, so they behave exactly like a text
anchor.

## Comments are per content state

If your project uses [content states](content-states.md), comments belong to
**one face**. A remark on the draft ("this paragraph needs a source") is not a
remark on the published version, and would be misleading if shown there — the
published text may not even contain the quoted words.

The default face keeps the bare id, so a project that never uses faces sees
exactly the storage layout above, and comments written before you adopted faces
keep working unchanged.

## The HTTP surface

```text
GET    /api/v1/_comments/{type}/{id}              list a target's comments
POST   /api/v1/_comments/{type}/{id}              add one
PATCH  /api/v1/_comments/{type}/{id}/{commentID}  edit the body / resolve
DELETE /api/v1/_comments/{type}/{id}/{commentID}  remove one
POST   /api/v1/_comments/{type}/{id}/resolve      check whether a selection can be anchored
```

Add a `@face` suffix to the id to address one content state:
`/api/v1/_comments/ticket/TKT-001@draft`.

`id` and `author` in a create body are **ignored** — the server mints the ID and
writes the author from the request principal. That is not a validation nicety:
accepting a client-supplied ID would let a caller overwrite someone else's
comment by reusing it.

## What comments deliberately do not do

- **No entity type.** Comments never enter `store.Store`, `entitymanager`, the
  audit log or `/_schema`.
- **No versioning.** A comment's edit history is not kept.
- **No search indexing.** Comments are not returned by `/_search`.
- **No threading.** A reply is a separate comment sharing an anchor; they render
  together because they resolve to the same place.

## Limits

| Limit | Value |
|---|---|
| Comment body | 16 KB, no control characters except tab and newline |
| Comments per target | 500 |
| Anchored quote | 5 characters minimum, 2 KB maximum |

The quote minimum exists because a shorter selection cannot be re-located
reliably — accepting one would mint a comment that detaches on the next edit.
