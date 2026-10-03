---
id: GUIDE-content-states
type: guide
title: "How To Publish Content with Faces and Worlds"
status: published
order: 5
audience: intermediate
summary: "Give an entity type several content states (draft, published, translations), declare worlds that pick one face per reader, and publish through a guarded copy"
---

## Introduction

Most content systems eventually need the same entity to exist in more than one
version. A policy is drafted, reviewed, and then published, and readers must
see only the published text. A guide exists in English and Dutch, and a Dutch
reader should get the translation when there is one and the English text when
there is not. Bolting a `status` field onto the entity does not solve this:
every list, export, and search would have to remember to filter on it, and one
forgotten filter leaks a draft.

rela solves this with two declarations. A **face** is one content state of an
entity. A **world** is a named rule that picks one face per entity for a
reader, and it is the only place that choice is made. A world can also leave an
entity out entirely, which is how a `published` world hides an unpublished
draft. Reading a world is a permission of its own, and moving content from one
face to another is a declared, guarded operation rather than a field edit.

In this guide you will declare faces on an entity type, declare three worlds
over them, grant roles access to those worlds, define a `publish` copy that
moves a draft into its published face, configure the web app to open in the
published world, and verify the result over the HTTP API. When you are done,
readers of your project will see published content only, editors will see
drafts, and publishing will be a single authorized action.

## Prerequisites

To complete this guide, you will need:

- A rela project with a `schema.yaml`, a `data-entry.yaml`, and an `acl.yaml`.
  The [Getting Started guide](getting-started.md) creates one.
- Familiarity with entity types and relation types in `schema.yaml`. The
  [Metamodel Reference](metamodel.md) covers both.
- Familiarity with roles and grants in `acl.yaml`. The
  [ACL: Authorization Overview](acl-overview.md) explains how a grant is
  evaluated.
- A running `rela-server` and `curl`, for the verification steps. The
  [Data Entry Web App guide](data-entry.md) shows how to start the server.

The examples model a small handbook: a `policy` type that is drafted and
published, a `guide` type that is translated, and a `control` type that has
neither. A complete, runnable version of this project is checked into the
repository under `prototypes/worlds/project/`.

## Step 1 — Declaring Faces on an Entity Type

A type declares its faces with the `faces:` key. Each key is a face name, and
that name is how the face is addressed and stored.

Open `schema.yaml` and add faces to the `policy` type:

```yaml
entities:
  policy:
    label: Policy
    id_prefix: POL
    faces:
      draft:     { label: "Draft" }
      published: { label: "Published" }
    properties:
      title:  { type: string, required: true }
      owner:  { type: string }
```

`faces:` declares two content states. A policy's draft is `POL-1@draft` and its
published form is `POL-1@published`. Every face is spelled the same way: the
entity id, `@`, and the declared face name. No face is reachable as the bare
`POL-1`, because a type declaring `faces:` stores no row under the bare id.

The `label:` on each face is display text for the web app and has no effect on
resolution. When you omit it, the face name is shown instead. A face may also
carry `messages:`, the operator's own words about that face. There are two
keys, and they differ in who the sentence is about. `read_only` is about the
**reader** — shown on a page or form that reached this face while they may not
write it; without it the page shows no explanation, as for any other permission
denial. `notice` is about the **document** — shown on a detail page whatever
the reader may do with it:

```yaml
faces:
  draft:
    label: Draft
    messages:
      notice: 'This policy is a draft and has not been adopted.'
  published:
    label: Published
    messages:
      read_only: 'This is the adopted version. Edit the draft instead.'
```

The distinction matters because a draft is writable by definition, so it can
never satisfy `read_only`'s condition — marking a draft as not-yet-in-force is
what `notice` is for. Both may be declared on one face, and the page then shows
`notice` first.

A face's declared name is also its storage coordinate, so what you write in
`schema.yaml` is what you write in a URL and in an `acl.yaml` grant. There is
no face that is spelled one way and stored another, and no privileged face that
the bare id stands for. Adding `faces:` to a type that already holds data is
therefore a real migration, not a relabelling — the existing rows sit at the
bare coordinate, which now names no declared face, so you have to say which
face they became. The [Data Migration guide](data-migration.md) covers this.

Now add a `guide` type that uses faces for languages rather than for a
lifecycle:

```yaml
  guide:
    label: Guide
    id_prefix: GUIDE
    faces:
      en: { label: "English" }
      nl: { label: "Nederlands" }
    properties:
      title: { type: string, required: true }
```

Nothing about the mechanism changes. The faces are peers rather than stages,
and neither is privileged by where it is stored: `GUIDE-1@en` and `GUIDE-1@nl`
are two rows of equal standing. Which one a reader gets is a world's decision,
made in Step 2, not a property of the schema.

Finally, leave the `control` type without faces:

```yaml
  control:
    label: Control
    id_prefix: CTL
    properties:
      title: { type: string, required: true }
```

A type without `faces:` has exactly one state, and that state appears in every
world. Faces are opt-in per type, so adopting them for policies costs nothing
for controls.

Face names use a strict grammar: lowercase letters and digits in runs joined by
single hyphens, such as `draft`, `published`, or `in-review`. Uppercase
letters, underscores, a leading digit, doubled hyphens, and the `+` character
are rejected when the schema loads. The same grammar applies to world names,
because both reach URLs and `acl.yaml` grants.

You have declared which faces exist. Next you will declare the worlds that
choose between them.

## Step 2 — Declaring Worlds

A world is declared in a top-level `worlds:` block. It has two parts that
answer two different questions: `select:` says which face to prefer, and
`otherwise:` says what to do when an entity has none of the preferred faces.

Add three worlds to `schema.yaml`:

```yaml
worlds:
  published:
    select: published
    overrides:
      guide: [en]
    otherwise: exclude
    banner: "Published — this is what readers see"

  editorial:
    select: [draft, published]
    otherwise: default
    banner: "Editorial — drafts included"

  site-nl:
    select: [nl, en]
    otherwise: default
```

Each world resolves every entity to at most one face using three rules, in
order:

1. If the type declares no faces, the entity appears with its only state.
2. If the entity has a face that the world's chain names, the first such face
   in chain order is shown.
3. Otherwise, the world's `otherwise:` rule decides: `exclude` leaves the
   entity out of the world entirely, and `default` shows the entity's single
   unnamed state.

Rule 3's `default` is about types that have such a state to show. A type
declaring `faces:` stores every row under a face name and has no unnamed state,
so for those types `default` has nothing to substitute and behaves as
`exclude`. Write the chain so that it names a face every entity can be expected
to have, and reserve `otherwise:` for deciding what a genuinely faceless or
unmatched entity does.

Read the three worlds against those rules. The `published` world selects the
`published` face and excludes anything that lacks one, so a policy with no
published face is not greyed out or marked as a draft. It is absent, and that
absence is the publication bit. The `overrides:` key replaces the chain for
the `guide` type, because guides have no `published` face and would otherwise
all be excluded. For guides, the English face *is* the published form.

The `editorial` world prefers the draft and falls back to the published face.
Every policy has one or the other, so every policy appears, and a policy that
has been drafted but not yet published still shows its draft.

The `site-nl` world prefers Dutch and falls back to English. Its `otherwise:
default` means a guide with no translation is still readable rather than
missing.

The following table summarizes the keys a world accepts:

| Key | Meaning |
| --- | --- |
| `select` | The face to show, or an ordered list. The first face the entity has wins. A single name and a one-element list mean the same thing. |
| `overrides` | A map from entity type to a chain that replaces `select` for that type. It replaces the chain rather than extending it. |
| `otherwise` | **Required.** `exclude` or `default`. What happens to an entity whose type declares faces but that has none the chain names. |
| `banner` | Optional text the web app shows at the top of every page in this world. Empty shows no announcement. |
| `messages` | Optional. The web app's wording for what this world changes on a screen: `absent` (a detail page for an entity with no face here; placeholders `{face}`, `{world}`, `{title}`), `projection` (a list or board note; `{world}` only), `stand_in` (the badge on a row served a stand-in; `{face}`, `{world}`). An undeclared entry shows nothing. |
| `on_absent` | Optional. `redirect: <world>` sends a reader who opens an entity with no face here to that world instead of showing the page. |
| `primary_for` | Optional. Breaks a tie when two worlds lead with the same face for a type. See the Metamodel Reference. |
| `create` | The face a create issued from this world lands in. A faced type has no default row, so a create from a world without this key is refused. Not derived from `select`: a world heading the ADOPTED face would otherwise publish by the act of creating. |
| `edits` | Accepted and validated as a declared face name, but not used yet. |

`otherwise:` has no default and a world without it does not load. The two
values are opposites, and both are reasonable: a public world wants `exclude`,
an internal one usually wants `default`. Guessing wrong would mean a
`published` world quietly serving a draft, which is exactly the failure this
feature exists to prevent, so the schema has to say which one it means.

A schema that declares no worlds gets one generated world, named `default`. It
holds every entity once: a faceless type at its only state, a faced type at the
first face it has, in the order the type declares its faces. Once you declare
worlds, only they exist. There is no `default` world beside them, and naming
one anywhere in the configuration is a load error rather than a quiet
reference to some other world. The name `default` stays reserved, so no
declared world can take it. Declare a world for every axis you expect people to
browse.

The schema loader rejects a world that declares neither `select:` nor
`overrides:`, a chain naming a face no type declares, an override naming a type
that declares no faces, and a name that fails the grammar. All errors are
collected and reported together, so fix the whole list before restarting.

You now have three worlds over the same entities. Before you grant anyone
access to them, decide which relations belong to a face.

## Step 3 — Scoping Relations to a Face

Once an entity has several faces, each relation type has to say what its edges
attach to. The `scope:` key on a relation type has two values.

Add two relations to `schema.yaml`:

```yaml
relations:
  implements:
    description: A policy implements a control.
    from: [policy]
    to: [control]
    scope: content

  owned-by:
    description: A policy is owned by a team.
    from: [policy]
    to: [control]
    scope: identity
```

`scope: identity` is the default. The edge belongs to the entity as a whole
and is shared by every face. Ownership, containment, and membership are
identity facts: a draft does not get a different owner than its published text
by accident.

`scope: content` attaches the edge to one face on its **source** side. A draft
may implement a different set of controls than the published face does, and
when the draft is published, the copy definition you declare in Step 5 decides
whether those edges travel with it. The target of a relation is always the
entity, never one of its faces.

When a reader in a world looks at an entity's relations, both halves resolve
through that world. The edges are those of the face being shown. Each
neighbour is then resolved through the same world on its own, so a Dutch page
links to Dutch neighbours where they exist and to English ones where they do
not, and a `published` world drops links to controls that have no published
face.

Seen from the target, the same source can link from several faces.
`POL-1@draft` and `POL-1@published` both implementing `CTRL-1` are two edges.
On the control's page, an incoming relation widget groups these edges per
source face:

- It shows every edge whose source face you may read. The ACL decides this,
  not the world.
- It names the face on each row.
- It locks a row you may not change.
- When you add an edge, it offers only the faces you may create the edge
  from. The server decides this with the checks the save runs.

Removing the draft edge leaves the published one alone.

With the graph shape settled, you can now control who reads which world.

## Step 4 — Granting Access to Worlds and Faces

Reading a non-default world is a permission of its own. The `published` and
`editorial` worlds contain the same entities, and the difference between them
is precisely what needs authorizing.

Open `acl.yaml` and declare three roles:

```yaml
role_relations:
  member-of:
    requires_permission: manage-roles

roles:
  editor:
    read: ["*", "world:published", "world:editorial", "world:site-nl"]
    permissions: [manage-roles, publish-policy]
    create: ["*", "policy@draft", "guide@en", "guide@nl"]
    update: ["*", "policy@draft", "guide@en", "guide@nl"]
    delete: ["*", "policy@draft", "guide@en", "guide@nl"]

  reader:
    read: ["policy@published", "guide", "control", "world:published", "world:site-nl"]

  translator:
    read: ["*", "world:published", "world:site-nl"]
    update: ["guide@nl"]
```

Every write grant names a face. `*` is a wildcard over **types**, never over
faces, so `update: ["*"]` alone would leave `editor` unable to write a single
policy or guide. Listing the faces beside it is what makes the role work, and
it is also where the interesting omission lives: `editor` may write
`policy@draft` but not `policy@published`. The published face is reached only
through the guarded copy declared in Step 5, so no role holds a direct write on
what readers see.

The `translator` role names the face it writes for the same reason. `guide`
declares faces, so a bare `update: [guide]` would grant nothing — the point the
warning at the end of this step returns to. Naming `guide@nl` also says what
the role is for: a translator edits the Dutch text and leaves the English
original alone.

A `world:<name>` entry in a role's `read:` list grants the right to select that
world with `?world=`. The default world needs no such entry: any ordinary read
grant covers it. A `world:` entry must name a declared world. The empty name
and the `world:*` glob are rejected when the policy loads, because a glob would
silently absorb worlds declared later.

A world is a global lens, so a `world:` grant on a role conferred through a
relation (`role_relations`) opens the world for a principal who holds that role
through any relation to any entity. The per-entity and per-face grants then
decide what the world shows them. This is also why a relation conferring such
a role must carry `requires_permission`: without it, writing one edge would be
enough to open the world.

A `world:` grant selects a lens. It does not by itself keep a role away from
content. The `reader` role shows the grant that does: `policy@published` is a
**face-scoped read grant**. It gates every read path, including lists, single
entity reads, `?include=` neighbours, attachments, history, and search, so a
reader cannot reach a draft even in the default world. A denied face produces
the same not-found response as a missing one.

Under a world, the grant trims the candidates before the world ranks them: a
`policy@published` reader in a world that prefers `review` and falls back to
`published` is served the published face. The single-entity read, lists,
`?include=` neighbours, the links in a response's `relations`, relation
filters, and search all work this way. Views do not yet: a view drops a
neighbour whose preferred face is denied instead of falling through, which
shows less, never more. The world is a view onto the part of the graph the
reader may see. An entity with no readable face in the world is absent, and
that absence looks the same as an entity the world excludes.

Reads and writes default differently, and the difference is deliberate:

| Grant | Covers |
| --- | --- |
| `read: [policy]` | Every face of every policy |
| `read: [policy@published]` | The published face only |
| `read: ["*"]` | Every type, every face |
| `update: [policy]` | Nothing, on a faced type — see the warning below |
| `update: [policy@published]` | The published face only |
| `update: ["*"]` | Every type, unnamed state only |
| `rename: [policy]` | Renaming a policy, which moves every face |

A bare read grant covers every face because a world never serves the unnamed
state when its chain names a face. If a bare read grant covered only that
state, a role holding it would read nothing under any world. Writes address a
face by id and never pass through a world, so they can safely stay narrow. As a
consequence, adding `faces:` to a live type does not tighten existing read
grants. If a role must be kept away from drafts, name the face it may read, as
`reader` does.

**Warning:** A write grant names the face **as stored**, and a faced type
stores nothing at the bare coordinate. A bare `update: [policy]` would reach no
row at all, so it is a load error: `acl.yaml` is refused, and the message names
the grant and the face-qualified grants to write instead. Name each face the
role may write:

```yaml
editor:
  update: [policy@draft, policy@published]
```

A rename moves every face of an entity, including faces the renaming user
cannot read. So it is granted for the whole entity, never per face:

```yaml
editor:
  update: [policy@draft, policy@published]
  rename: [policy]
```

Face grants do not grant a rename, even on every face. A face-qualified
rename grant such as `rename: [policy@draft]` is a load error. `rename: ["*"]`
covers every type. On a faceless type, `update:` still grants a rename as
well. Global roles and local roles both count. A local role is conferred
through an identity-scoped relation, so it belongs to the whole entity.

The user type, group types and the relations the ACL walks for roles must stay
faceless and identity-scoped. The
[ACL: Security Hardening guide](acl-security.md#users-groups-and-role-relations-must-be-faceless)
lists the rules.

The `role_relations` block at the top is not optional once a non-default world
grant exists. A role that can read `world:editorial` is worth stealing, so a
policy that grants one while leaving the membership relation ungated is
**refused at load** rather than booted with a warning. One
`requires_permission` line closes the self-promotion path. Projects that
declare no worlds keep the previous warn-and-boot behaviour.

Run the audit after every change to `acl.yaml`:

```bash
rela acl audit
```

The audit reports a `read: [world:X]` grant naming a world the schema does not
declare and a `type@face` grant naming a face the type does not declare. Both
fail closed at runtime by matching nothing, so without the audit the only
symptom would be a denial nobody can explain.

Your roles now control who may read which world. Next you will define how
content moves from one face to another.

## Step 5 — Defining How Content Moves Between Faces

Ordinary writes address one face by name: an update or delete on
`POL-1@draft` touches the draft and nothing else. Moving content *between*
faces is a different operation, and `published` is written only through a
**copy definition** that names it as a target and carries its own permission
guard. This is what makes publishing an operation rather than a field edit:
there is no field that means published, only a face, and something authorized
has to put content in it.

### A create names its face

An entity of a faced type is born into one particular face, so the create
request says which:

```bash
curl -s -X POST http://localhost:8080/api/v1/policys \
  -H 'Content-Type: application/json' \
  -d '{"face": "draft", "properties": {"title": "Access Control"}}'
```

This writes `POL-1@draft` and nothing else. The new policy has no published
face until something publishes it, which is the state the `published` world
reads as "not published".

Omitting `face` is refused with `422` and the error code `face_required`, as is
naming a face the type does not declare. There is no default: guessing one
would mean a create silently landing in whichever face the schema happened to
list first, and for a lifecycle axis the wrong guess writes straight into the
readers' view. Naming a face for a type that declares none is refused too,
because that request describes a row that cannot exist.

The body is decoded strictly, so a misspelled key such as `"faces"` is a `400`
rather than a silently ignored field. A request that means to name a face and
fails to spell it must not look like a request that named none.

A Lua script names the face the same way, in a trailing options table:

```lua
rela.create_entity("policy", {title = "Access Control"}, "", nil,
                   { face = "draft" })
```

The script names the face **directly** rather than naming a world. A world
resolves through a chain and may answer with a fallback, but a write has to
name the row it changes — so the two are deliberately different vocabularies.
See the [Lua scripting guide](lua-scripting.md) for the full options table and
for how the other write bindings address a face.

Add a `copies:` block to `schema.yaml`:

```yaml
copies:
  publish:
    from: policy@draft
    to: policy@published
    label: Publish
    fields: all
    relations:
      implements: replace
    guard:
      permission: publish-policy
```

`from:` and `to:` address a face as `type` or `type@face`. `fields: all`
copies every declared property and the body, which is the full-replace
"promote" case. `relations: {implements: replace}` swaps the published face's
`implements` edges for the draft's. `label:` is the text the web app puts on
the button, falling back to the definition name when omitted.

`guard.permission` names the ACL permission a caller must hold on the source
entity. The `editor` role from Step 4 holds `publish-policy`, so editors can
publish. The permission is resolved per entity, so a role conferred through an
ownership relation satisfies it without a global grant.

For a guarded copy **between two faces of one entity**, the guard is the whole
write check: the caller needs the guard permission and nothing else, in either
direction. The operator wrote both endpoints in `schema.yaml`, so the caller
chooses nothing, and the permission names exactly who may move that content.

Every copy that names a face in `to:` must carry a `guard.permission`, and the
guard applies uniformly: a `publish` and a `revert` from `policy@published` back
to `policy@draft` are guarded on the same terms, because both write a declared
face. Requiring `update` as well would defeat the point, since the same grant
makes the face editable by hand.

Everything else needs the ordinary write grant:

- A copy whose two endpoints are the **same face** (`from: policy`,
  `to: policy`). It moves nothing between faces, so it is an ordinary in-place
  edit however it is guarded. Such a copy names no face in `to:`, so it needs
  no guard to declare.
- A copy into a **different entity**, guard or not, because there the caller
  names the target: `create` when it does not exist yet, `update` when it does.
  A target that exists under another type is refused.

The caller always needs read access to the source face. A guard says "you may
perform this promotion", not "you may read this document".

`fields: all` is a **full replace** of the target face, not a merge. A
same-entity copy reads the source unredacted, so it writes every property
including ones the caller cannot see, and any property that exists only on the
target is dropped. For a promote that is the point — publishing the whole
document is the operation. Carry any target-only property on the source face,
or map fields explicitly instead.

The following table lists the keys a copy accepts:

| Key | Meaning |
| --- | --- |
| `from` | Source face, as `type` or `type@face`. |
| `to` | Target face. When it names a face, `guard:` becomes mandatory. |
| `label` | Display text for the action. Plain text, no interpolation. |
| `on_success` | Optional. `message:` is the confirmation the web app shows (default: the copy's label; `{face}` names the face written); `landing:` is where it goes afterwards: `written` (default), `stay`, `{world: name}` or `{face: name}`. |
| `fields` | `all` to copy every property, or a map of target property to source expression. A copy between different types requires an explicit map. |
| `relations` | A map of relation type to `merge` (add missing edges) or `replace` (swap the target face's edges). Only `scope: content` relation types can be listed. An omitted type is not copied. |
| `guard.permission` | The permission required on the source entity. **Required** whenever `to` names a face. On a copy between two faces of one entity it replaces the ordinary `update`/`create` check. |

The loader enforces several rules so that a definition that resolves wrongly
never reaches a reader. A copy into a named face without a `guard.permission`
is refused: an unguarded definition would open the face to anyone who can name
the copy. A copy of an identity-scoped relation is refused, because such an
edge is shared by every face and copying it could duplicate an edge that
confers roles. `guard.when` is accepted by the parser but refused at load with
a message asking you to remove it, because a condition that is written but
never evaluated is worse than none.

A copy never creates an edge the caller could not create by hand. It does
not copy an edge to an entity the caller cannot read, and it does not copy an
edge that the relation affordances or the ACL refuse on the target face. Such
an edge is skipped and the copy succeeds without it; the response does not
mention it. A copy between two faces of one entity with a `guard.permission`
skips the ACL check on its edges, as it does on the face itself. `replace`
removes only the target face's edges to entities the caller can read.

A copy runs as one store transaction and is audited after the commit. On the
PostgreSQL backend a failed copy rolls back completely. On the filesystem and
in-memory backends the transaction is a write lock only, so a copy that fails
part-way can leave a partially written target face.

**Note:** A copy between faces of the same entity runs with elevated
visibility: properties the caller may not read travel with the entity, because
the same policy governs them on the target face. A copy into a *different*
entity reads through the caller's own visibility, and `fields: all` is refused
for it, since copying a redacted view of an entity would destroy the fields
the caller could not see. Cross-entity copies are supported by the API but
have no button in the web app.

You have declared the schema side of the feature. Now configure how the web
app presents it.

## Step 6 — Configuring the Web App

The web app reads the world from the URL and applies the schema's default world
when the URL names none. Set it with a top-level key in `schema.yaml`:

```yaml
default_world: published
```

`default_world` names the world a request lands in when it carries no
`?world=`. For a handbook the world to land in is `published`, so readers see
the adopted text and editors reach drafts deliberately by selecting
`editorial`. The key is required when the schema declares more than one
world, and a schema without it fails to load. Otherwise reordering `worlds:`
would change the default world without anyone noticing. With a single declared
world the key may be left out, and that world is the default. Lua scripts, the
MCP server, the CLI, scheduled tasks and validation read in the same world.

`app.default_world` in `data-entry.yaml` is the older spelling. It is now a
deprecated alias that must name the same world as the schema, and a
contradiction fails the load. It does not stand in for the schema key: move
the value to `schema.yaml` and remove it from `data-entry.yaml`.

`default_world` is presentation, not policy. It grants nothing: the world's
read grant is re-checked on every request exactly as for an explicit `?world=`,
so pointing it at a world a role may not read yields that world's ordinary
empty result. The server applies it to `curl` and to the browser alike, but
only on read requests and only on routes that can serve a world. Naming an
undeclared world here is a startup error.

Next, tell the policies list where its create button should land:

```yaml
lists:
  policies:
    entity_type: policy
    title: "Policies"
    create_form: new_policy
    create_world: editorial
    columns:
      - { property: title, link: detail }
      - { property: owner }
```

`create_world` governs where the author lands after a create. The list is
shown in the `published` world, where a newly drafted policy has no face, so
without it the author would be redirected to a page saying their new policy is
not in this world. `create_world: editorial` opens the form in the editorial
world and carries that world onto the post-create redirect, so the author lands
on the draft they made. It must name a declared world.

With these two keys set, the web app behaves as follows in a non-default world.
One rule governs every sentence it shows about worlds and faces: **the words are
the operator's, or there are none.** The app has no text of its own for any of
this, because "face", "world" and "default" are storage vocabulary a reader
never chose.

- The world is part of the URL as `?world=<name>`, so a world-bound page is a
  shareable link. Switching worlds resets pagination and adds a history entry.
- A page shows the world's `banner:` text when one is declared. On a list or
  board of a type that declares faces it also shows the world's
  `messages.projection`, if declared.
- Every write goes to the **address** of the row on screen, face included:
  what you look at is what you edit is what you save. A detail page whose
  entity resolved to its published face opens its edit form on
  `POL-1@published`, and a page showing the draft opens it on `POL-1@draft`.
  Whether
  a write is offered is the server's `_actions` verdict for that face, so an
  editor looking at an adopted text sees no Edit button (no grant names the
  published face), and the same editor looking at the draft, in whichever
  world, edits the draft. A page showing a face the reader may not write
  carries that face's `messages.read_only` if one is declared, and otherwise
  nothing; the entity's other faces are one click away through the face
  switcher. A page showing a face that declares `messages.notice` carries that
  sentence whether or not the reader may write it — it describes the document,
  not the permission — and both appear together, `notice` first, when a face
  declares both.
- A **View Published** button, or a menu when there are several faces, lets
  the reader switch to the entity's other faces by address, staying in the
  world they are browsing. It renders on every screen that has faces,
  including the default world.
- A row or card served a **stand-in** (an entity resolved through
  `otherwise: default`, or through a later entry in the chain than the first)
  carries a badge with the world's `messages.stand_in` text, typically
  `{face}`. A first-choice hit shows no badge, and a world that declares no
  text shows none at all.
- An entity that exists but has no face in the world renders one of the faces
  the reader may see, with the world's `messages.absent` if declared. With
  `on_absent: {redirect: <world>}` the app navigates to that world instead.

A policy's detail page shows the **Publish** button when the caller holds
`publish-policy` and the draft is on screen, in whichever world. A caller
without the permission sees no button rather than a disabled one. After a
successful publish, the app shows the copy's `on_success.message` (or just its
label) and lands per `on_success.landing`: on the face it wrote by default,
so the draft is then one click away through the face switcher.

Two more `data-entry.yaml` surfaces take a world. A next-action source can set
`source_world` to decide which world its candidate query runs in and
`visible_worlds` to decide in which worlds the suggestion is displayed. A
kanban board is a projection too: each card is one entity at the face the
world resolved, and an entity with no face in the world has no card. The
[Data Entry Web App guide](data-entry.md#worlds-in-the-web-app-and-api)
documents these keys.

Start the server so that you can verify the configuration:

```bash
rela-server -project /path/to/project
```

The server compiles every world when it starts. A schema error in `faces:`,
`worlds:`, or `copies:` stops the start with the full list of problems.

## Step 7 — Reading a World over the API

The HTTP API selects a world with the `?world=` query parameter on the entity
list and the single-entity read. This step uses `curl` against a server on
`localhost:8080`; adjust the host and any authentication your deployment
requires.

First, discover the declared worlds:

```bash
curl -s http://localhost:8080/api/v1/_schema
```

The response includes a `worlds` block:

```json
"worlds": {
  "default":   { "readable": true, "default": true },
  "published": { "select": ["published"], "overrides": { "guide": ["en"] },
                 "otherwise": "exclude", "banner": "Published — this is what readers see",
                 "readable": true },
  "editorial": { "select": ["draft", "published"], "otherwise": "default",
                 "banner": "Editorial — drafts included", "readable": false },
  "site-nl":   { "select": ["nl", "en"], "otherwise": "default", "readable": true }
}
```

Every declared world is listed for every caller, because world names are
configuration in your repository rather than secrets. `readable` says whether
*this* caller may select the world. The same response lists each type's
declared faces under `entities.<type>.faces`, and the `default_world` you
configured.

Now list the policies a reader sees:

```bash
curl -s "http://localhost:8080/api/v1/policys?world=published"
```

The list contains only policies that have a published face. Omitting the
parameter serves the default world named by `default_world`. A schema that
declares worlds has no world named `default`, so `?world=default` is answered
with `400 unknown_world`. Only a schema that
declares no worlds has the generated `default` world, which serves each faced
type's faces in declaration order.

Read one entity in a world:

```bash
curl -s "http://localhost:8080/api/v1/guides/GUIDE-2?world=site-nl"
```

A single-entity response carries the provenance of the face it served:

```json
"_world": { "name": "site-nl", "face": "en", "via": "chain", "chain_position": 1 }
```

`face` is the name of the face that was served, which is also the coordinate
it is stored at. `via` names the resolution rule: `unscoped` for a type without
faces or the default world, `chain` when a face the world selects exists, and
`fallback-default` when the `otherwise: default` rule substituted a faceless
type's single state. `chain_position` is the
zero-based index of the served face in the world's chain and is present only
for `via: chain`. Position `0` is the world's first choice. Any later position
is a stand-in, as in this example, where `site-nl` asked for Dutch and served
English. The bytes alone do not tell you which one you got, which is why the
field exists.

The same response carries two affordance lists. `_faces` names the entity's
other faces that the caller may read, each with its stored coordinate and
label, so a client can offer a way to the published text or to a translation. `_copies` lists the copy
definitions whose `from:` matches the face being served, each with an
`allowed` verdict computed by the same authorization path the invoke uses.
`allowed` is a hint for rendering, never a boundary: the invoke re-authorizes.

Publish a policy by invoking the copy by name:

```bash
curl -s -X POST http://localhost:8080/api/v1/_copies/publish \
  -H 'Content-Type: application/json' \
  -d '{"source_id": "POL-1"}'
```

A request names a definition and a source. It can never describe a mapping,
which is what keeps the guard meaningful. A successful invoke returns the
target that was written:

```json
{ "definition": "publish", "entityId": "POL-1", "face": "published", "created": true }
```

`created` is `true` the first time the face comes into existence and `false`
when a copy overwrites an existing face. A caller without `publish-policy`
receives a `403` that names the missing permission. A source the caller may not
read produces the same `404` as a source that does not exist.

The following table lists the responses the world parameter can produce:

| Situation | Response |
| --- | --- |
| `?world=` names a world the schema does not declare | `400 unknown_world`, naming the world |
| `?world=` appears more than once | `400 duplicate_world` |
| `?world=` on a `POST`, `PATCH`, `PUT`, or `DELETE` | `422 world_read_only` |
| `?world=` on a route that cannot serve a world | `422 world_unsupported` |
| A declared world the caller may not read | An empty list, or a `404` for one entity, identical to a world holding nothing readable |
| An entity that has no face in the world | Omitted from lists; `404` from the single-entity read; `200` with `_world_absent: true` from the entity view |

Two of these deserve a closer look. An undeclared world is a named `400`
because the name is configuration, and telling the operator which name is
missing is more useful than silence. A world the caller may not read is an
empty result rather than a `403`, because what a world contains is exactly the
secret this feature keeps, and a `403` would confirm that there is something
to hide.

A world reaches the following routes:

| Route | World-scoped |
| --- | --- |
| `/api/v1/{plural}` and `/api/v1/{plural}/{id}` | Yes, including `?q=` search and `?include=` neighbours |
| `/api/v1/_views/{type}/{id}` | Yes. A view's `where:` clauses evaluate against the resolved face |
| `/api/v1/_history/{type}/{id}` | Yes. Versioning is per face on the database backends, so the history is the served face's. `{id}` may be `ID@face` |
| `/api/v1/_next_action` | Yes, as the display world for `visible_worlds` |
| `/api/v1/_search` | Yes. The command palette, search page and entity picker send the page's world. Dashboard cards count in the default world |
| `/api/v1/_position` | Yes. Prev/next within a search or list runs in the same world as the results it steps through |
| Documents, feeds, analysis, sync, attachments, export, relation sub-resources | No. An explicit `?world=` is refused with `422 world_unsupported` |

A history response names the face it belongs to and how that face was chosen:
`via` is `chain` (a face the world asked for, with `chain_position` giving
which one), `fallback-default` (no face in the chain existed, so the default
stood in), or `unscoped` (the world says nothing about this type). The rule
matters more here than on a reading surface. A world with `otherwise: default`
answers a missing face with a stand-in, which is the right answer for a reader
and a misleading one for a timeline: without `via`, a history labeled only by
face looks like the one you asked for. It is the same label the entity endpoint
returns, computed the same way, so the two cannot disagree about one
resolution.

Analysis is deliberately unscoped. It reports on the health of the whole graph
a caller may read, and a world that hides a broken draft would make the graph
look clean precisely where it is not.

Search under a world matches the text of the face the world resolves for the
caller, and an entity the world excludes has nothing to match. Searching the
`published` world for a word that appears only in a draft returns exactly what
searching for a nonsense word returns. The same holds for a face the caller may
not read: search never matches its text, and serves the next readable face in
the world instead, as the other read paths do.

You have verified the schema, the grants, and the copy from outside the web
app. The last step checks the stored data itself.

## Step 8 — Verifying the Setup

Three commands check the parts of the configuration that do not fail at
startup.

First, validate the project:

```bash
rela validate
```

Validation loads the schema, which compiles every world and every copy
definition, so a face name that fails the grammar or a copy into a guarded
face without a guard is reported here. Custom validation rules run against
every face of an entity unless the rule is narrowed with a `faces:` key. The
[Metamodel Reference](metamodel.md#faces--scoping-a-rule-to-content-states)
describes that key.

Next, audit the access policy:

```bash
rela acl audit
```

Look for `B10-undeclared-world` and `B11-undeclared-face` findings, which
mark grants that will silently match nothing. The mistake warned about in
Step 4, a bare `update: [policy]` on a type that declares faces, never reaches
the audit: `acl.yaml` fails to load and the error names the grant.

Finally, check the stored faces against the schema:

```bash
rela analyze states
```

This reports rows stored under a face no type declares, for example after a
face was renamed or removed from `faces:`; rows stranded at the bare id on a
type that declares `faces:` — the shape left behind when a type gains faces
while its existing rows stay at the coordinate that now names no declared face;
and rows whose entity type the schema does not define at all.

A faced row whose bare sibling is missing is **not** reported: that is the
ordinary shape of a faced entity, not a fault.

It detects only. To move rows between faces, use the `rename_face` step of the
[data migration system](data-migration.md#renaming-a-content-state); to adopt
bare rows into a face, use `migrate_face`.

If your project uses the PostgreSQL or SQLite backend, each face keeps its own
version history. Editing the draft versions `POL-1@draft`, and invoking
`publish` versions `POL-1@published`. The history page in the web app names the
face it shows, and restoring a version restores that face only.

Every history surface takes an address:

- `GET /api/v1/_history/policy/POL-1@draft` reads the draft's history, and
  `POST .../POL-1@draft/3/restore` restores it. A bare id is resolved the way
  the entity endpoint resolves it.
- `rela history POL-1@draft`, `rela restore POL-1@draft 3` and
  `rela history-purge POL-1@draft ...` name the face on the command line. A
  bare id of a faced entity is refused, and the error lists its faces.
- Restoring a version of a deleted face re-creates that face at the same id.
  The restore is authorized as a create on `policy@draft`, or as an update when
  the face still exists. If another writer re-creates the face during the
  restore, the restore is refused with a conflict and the new face is kept.
- A purge reaches one face. `--all` erases the draft's history and leaves the
  published face's history intact.

### Attachments and export on a face

A file belongs to the face it was uploaded on. Attaching a file to
`POL-1@draft` does not show it on `POL-1@published`. Every attachment surface
takes an address:

- The HTTP API: `PUT /api/v1/policies/POL-1@draft/_attachments/evidence`.
- The command line: `rela attach POL-1@draft evidence.pdf`. A bare id of a
  faced entity is refused, and the message names its faces.
- The MCP tools: `"id": "POL-1@draft"`.

The bytes are stored once per entity. A face lists and serves only the files
its own file property names. A copy that carries the file property, such as
`publish` with `fields: all`, gives the target face a reference to the same
bytes, so nothing is duplicated. A copy can only carry files the source face
already references.

File names are unique per face, not per entity. Every upload gets its own
storage key, which the file property records next to the name. So two faces
can each hold a `report.pdf` with different contents. An upload behaves
exactly as if no other face held a file of that name: it is never renamed
because of another face, so it reveals nothing about files the writer
cannot read.

Deleting a file from one face keeps the bytes while another face still
references them. The last reference takes the bytes with it, and so does
deleting a face that held the last reference.

Only the attachment surfaces, copies, sync and data migrations change a file
property. An ordinary update that changes or clears a file value is refused
with `422`, because the value decides which files a face may serve. Restoring
a version keeps the face's current file values, and duplicating an entity
leaves them out. An automation cannot write a file property either: a
schema whose automation names one in `set:` or `create_entity` fails to
load.

Export takes an address as well: `GET /api/v1/policies/POL-1@draft/_export`
exports the draft. The read grant for that face applies, so a reader granted
`policy@published` gets a not-found for the draft.

## How a Write Finds Its Face

Every create names a face: the HTTP API takes `face` or `world` in the body,
the web app's create form asks for a face when its world declares no
`create:`, `rela create` takes `--face`, the MCP `create_entity` tool takes
`face`, and the Lua `create_entity` binding takes `{ face = ... }`. A create
on a faced type that names none is refused with `422 face_required`, and the
error lists the faces the type declares in `faces`.

An update, delete, attach or content-scoped relation write on a faced type
names the face it changes (`POL-1@draft`). A bare id is refused with
`422 face_required`, even when only one face exists, and the error names the
faces the caller may read, for example
`POL-1 has faces; address one: POL-1@draft, POL-1@published`. A write never
lands on whichever face a world ranks first, and a write that works today
does not start failing when a second face is published. On a faceless type a
bare id names its one face. Deleting the last face of an entity deletes the
entity.

An identity-scoped relation belongs to the entity, not to a face, so a bare
id is accepted for it, and a faced address writes the same edge.

A content-scoped relation belongs to one face of its source. An edge created
from the target's side (an incoming edge) therefore names the source's face
in the body: `{"id": "POL-1@draft", "direction": "incoming"}`, or
`POL-1@draft` in a relations PATCH. An edge the source already has keeps its
face, so a PATCH may list it by bare id. The entity manager refuses a
content-scoped edge from a faced source that names no face, whichever client
sends it.

A calendar client names the face too: a faced to-do is served over CalDAV
under its face's address (`task--TSK-1@draft@rela.ics`). When the world
starts serving another face, the client sees one to-do removed and another
added.

Some writes concern the whole entity rather than one face. Their outcome
depends only on your grants and on the faces you can read. It never depends on
a face hidden from you, so a refusal does not reveal that one exists.

- A **rename** moves every face, hidden ones included. It needs a `rename:`
  grant on the type. A user with the grant who can read no face of the entity
  gets the same `404` as for an entity that does not exist. The web app offers
  the rename on the same terms.
- An **identity-scoped relation** from a faced entity needs the write grant on
  every face the type declares, whether or not the entity stores that face. A
  `relation_grants:` permission is the alternative, as for any relation. An
  affordance `when:` is evaluated on the faces you can read. A face you cannot
  read allows the relation only if the grant has no `when:`. The web app's
  `linkable` flag follows the same rule.
- Deleting a face deletes the entity when no other face you can read remains.
  The delete is then checked as an entity delete: it needs `cascade` when the
  entity has relations, and each identity-scoped and incoming edge must be
  deletable. When a face you cannot read still exists, rela keeps the entity,
  that face and those edges, and deletes only the face you named and its own
  edges. The answer you get is the same either way.
- A `rela-docs` `assert-acl` claim about a rename is decided the same way and
  refuses `face=`; a delete claim with no face covers the family.

## Upgrading from a Release Without Implicit Faces

- `?world=default` is a `400 unknown_world` on a schema that declares worlds.
  The web app drops an unknown `?world=` from the URL and lands in the default
  world, so old bookmarks keep working.
- Every route reads in the default world, including routes that refuse an
  explicit `?world=`.
- `default_world` is required when the schema declares more than one world.
  A schema without it fails to load. Add `default_world: <world>` to
  `schema.yaml`; before this release the first declared world was used.
- `deny_worlds` naming the default world is refused at load. To keep a client
  away from content, name the faces in `read:` or use `deny_read`.
- The deprecated `app.default_world` in `data-entry.yaml` must name the same
  world as the schema, or the load fails. It does not satisfy the
  `default_world` requirement; move it to `schema.yaml`.
- A bare-id write on a faced type is refused with `face_required`, except
  for an identity-scoped relation. Name the face. This covers updates,
  deletes, attachments and content-scoped relations, in the HTTP API, MCP,
  the command line and Lua. A bare-id delete no longer deletes every face;
  delete each face, and the last one deletes the entity.
- A content-scoped relation from a faced entity must name the source face:
  a Lua `create_relation` passes `opts.face`, and an incoming edge names the
  source as `ID@face`. A content-scoped edge stored with no face before the
  upgrade can still be deleted.
- CalDAV and calendar-feed UIDs of a faced entity carry its face, so a
  calendar client sees each faced to-do or event once removed and re-added
  after the upgrade.
- A `face_required` error now carries `faces`.
- A list's collection `_actions` carries `create@<face>` for a faced type,
  and `create` is true when any face is creatable.
- `/api/v1/_schema` carries `world_order`, and each world carries its
  `create` face.
- A webhook whose `find.type` is faced must name `find.face` (see
  [Webhooks](webhooks.md#types-with-faces)).
- The derived static-query indexes are keyed on the face and are rebuilt on
  the first start.
- `rela acl audit` reports a grant on an undeclared world as finding
  `B10-undeclared-world`.
- A rename of a faced entity needs a `rename:` grant on its type; update
  grants on every face no longer grant it. Add `rename: [policy]` (with your
  type) to each role that renames. `rename: [policy@draft]` is refused at
  load.
- An identity-scoped relation from a faced entity is checked against every
  face its type declares, not only the faces the entity stores. A role that
  writes such relations needs the grant on each declared face, or a
  `relation_grants:` permission. An affordance relation grant with `when:`
  no longer allows such a relation when the caller cannot read every face of
  the entity.
- A relation write naming an entity or face the caller cannot read returns
  `422 target_not_found`, as for one that does not exist. A write naming an
  absent entity may now get this `422` where it got a `403` before, for
  example under `--read-only`.
- Deleting the last face you can read is checked as an entity delete, even
  when a face you cannot read still exists.
- A copy skips an edge the caller could not create by hand, where it copied
  it or refused the whole copy with a `403` before. This covers an edge to an
  entity the caller cannot read and an edge the relation affordances or the
  ACL refuse. `replace` keeps the target face's edges to entities the caller
  cannot read.

## What Worlds Do Not Cover Yet

The limits below are deliberate. A surface joins the world-aware set only when
its whole read path has been scoped and tested, so widening the set is a
visible change rather than a forgotten call site.

- The command-line interface has no `--world` flag, and `rela list`, `rela
  show`, and the export commands read the default world. The MCP server, Lua
  scripts, the scheduler and mail read the default world as well.
- Documents, calendar feeds, sync, attachments, exports, and the relation
  sub-resources of an entity refuse a world.
- Restoring a version under a world is refused, like every other write with a
  world. Restoring a version onto a face that still exists works; restoring a
  face that was **deleted** is refused for a faced type, because a version
  snapshot does not yet report which face it captured.
- `guard.when` on a copy and `edits:` on a world are parsed but not
  implemented. The first is refused at load, the second is accepted and
  ignored.
- Version history is available on the PostgreSQL and SQLite backends only.

## Conclusion

You declared faces on an entity type, declared worlds that select one face per
reader, scoped relations to a face or to the entity, granted roles the right to
read specific worlds and faces, defined a guarded `publish` copy, configured
the web app to open in the published world, and verified the result over the
API. Readers of your project now see published content only, editors see
drafts, and publishing is one authorized action with its own audit record.

From here, you can add a `review` face and a world that prefers it to preview
pending changes, add translation copies between language faces, or scope
validation rules and automations to particular faces. The
[Metamodel Reference](metamodel.md#content-states-and-worlds) lists every key
these declarations accept, and the [ACL: Security Hardening](acl-security.md)
guide covers the security reasoning behind world and face grants in depth.
