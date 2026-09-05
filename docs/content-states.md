<!-- This file is auto-generated from docs-project/entities/. Do not edit directly. -->

# How To Publish Content with Faces and Worlds

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
draft. Reading a world is a permission of its own, and writing a non-default
face is a declared, guarded operation rather than a field edit.

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
`bare_face:` says which of those names the bare entity id addresses.

Open `schema.yaml` and add faces to the `policy` type:

```yaml
entities:
  policy:
    label: Policy
    id_prefix: POL
    bare_face: draft
    faces:
      draft:     { label: "Draft" }
      published: { label: "Published" }
    properties:
      title:  { type: string, required: true }
      owner:  { type: string }
```

`faces:` declares two content states. `bare_face: draft` says that `POL-1` and
`POL-1@draft` are the same row, so a newly created policy *is* its own draft.
The `label:` on each face is display text for the web app and has no effect on
resolution. When you omit it, the face name is shown instead. A face may also
carry `messages: {read_only: "..."}`, the sentence the web app shows on a page
or form that reached this face while the reader may not write it; without it
the page shows no explanation, as for any other permission denial.

`bare_face:` names a row that already exists. Every entity has one row stored
under its bare id, whether or not the type declares faces, so adding `faces:`
to a type that already holds data migrates nothing. If you omit `bare_face:`,
every declared face becomes a separate suffixed row and the entity's own row
has no name. That is legal but rarely what you mean.

Now add a `guide` type that uses faces for languages rather than for a
lifecycle:

```yaml
  guide:
    label: Guide
    id_prefix: GUIDE
    bare_face: en
    faces:
      en: { label: "English" }
      nl: { label: "Nederlands" }
    properties:
      title: { type: string, required: true }
```

Nothing about the mechanism changes. The faces are peers rather than stages,
and English is the one a bare id addresses because that is the language the
handbook is written in first.

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
   entity out of the world entirely, and `default` shows the bare face.

Read the three worlds against those rules. The `published` world selects the
`published` face and excludes anything that lacks one, so a policy with no
published face is not greyed out or marked as a draft. It is absent, and that
absence is the publication bit. The `overrides:` key replaces the chain for
the `guide` type, because guides have no `published` face and would otherwise
all be excluded. For guides, the English face *is* the published form.

The `editorial` world prefers the draft and falls back to the published face,
then to the bare face. Because `draft` is the bare face for policies, every
policy matches at the first choice.

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
| `messages` | Optional. The web app's wording for what this world changes on a screen: `absent` (a detail page for an entity with no face here), `projection` (a list or board note), `stand_in` (the badge on a row served a stand-in). Placeholders `{face}`, `{bare_face}`, `{world}`, `{title}`. An undeclared entry shows nothing. |
| `on_absent` | Optional. `redirect: <world>` sends a reader who opens an entity with no face here to that world instead of showing the page. |
| `primary_for` | Optional. Breaks a tie when two worlds lead with the same face for a type. See the Metamodel Reference. |
| `edits` | Accepted and validated as a declared face name, but not used yet. |

`otherwise:` has no default and a world without it does not load. The two
values are opposites, and both are reasonable: a public world wants `exclude`,
an internal one usually wants `default`. Guessing wrong would mean a
`published` world quietly serving a draft, which is exactly the failure this
feature exists to prevent, so the schema has to say which one it means.

Every project also has an implicit **default world**. It applies no
resolution: every entity appears with its bare face, exactly as a project
without worlds behaves. It needs no declaration, and the name `default` is
reserved so nothing can shadow it.

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
    create: ["*"]
    update: ["*"]
    delete: ["*"]

  reader:
    read: ["policy@published", "guide", "control", "world:published", "world:site-nl"]

  translator:
    read: ["*", "world:published", "world:site-nl"]
    update: [guide]
```

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
`published` is served the published face, on lists and on the single-entity
read alike. The world is a view onto the part of the graph the reader may see.

Reads and writes default differently, and the difference is deliberate:

| Grant | Covers |
| --- | --- |
| `read: [policy]` | Every face of every policy |
| `read: [policy@published]` | The published face only |
| `read: ["*"]` | Every type, every face |
| `update: [policy]` | The bare face only |
| `update: [policy@published]` | The published face only |
| `update: ["*"]` | Every type, bare face only |

A bare read grant covers every face because a world never serves the bare face
when its chain names another. If a bare read grant covered only the bare face,
a role holding it would read nothing under any world. Writes address a face by
id and never pass through a world, so they can safely stay narrow. As a
consequence, adding `faces:` to a live type does not tighten existing read
grants. If a role must be kept away from drafts, name the face it may read, as
`reader` does.

**Warning:** A grant names the face **as stored**. With `bare_face: draft`,
the draft row lives at the bare id, so the grant that reaches it is the bare
`update: [policy]`. Writing `update: [policy@draft]` matches nothing and
denies the face it was meant to allow. `rela acl audit` reports it as
`B12-bare-face-named`.

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

Ordinary writes address the bare face: a create, update, or delete on `POL-1`
touches the draft, because `draft` is the bare face. A non-bare face such as
`published` is written only through a **copy definition** that names it as a
target and carries its own permission guard. This is what makes publishing an
operation rather than a field edit: there is no field that means published,
only a face, and something authorized has to put content in it.

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

The guard is the whole write check only for a copy into a non-bare face. A copy
whose target is the **bare** face (a `revert` from `policy@published` back to
`policy@draft`, say) is an edit of the draft, so the caller also needs the
ordinary `update` grant on it. A copy into a different entity needs `create` on
the target when it does not exist yet and `update` when it does, and a target
that exists under another type is refused.

The following table lists the keys a copy accepts:

| Key | Meaning |
| --- | --- |
| `from` | Source face, as `type` or `type@face`. |
| `to` | Target face. When it names a non-bare face, `guard:` becomes mandatory. |
| `label` | Display text for the action. Plain text, no interpolation. |
| `on_success` | Optional. `message:` is the confirmation the web app shows (default: the copy's label; `{face}` names the face written); `landing:` is where it goes afterwards: `written` (default), `stay`, `{world: name}` or `{face: name}`. |
| `fields` | `all` to copy every property, or a map of target property to source expression. A copy between different types requires an explicit map. |
| `relations` | A map of relation type to `merge` (add missing edges) or `replace` (swap the target face's edges). Only `scope: content` relation types can be listed. An omitted type is not copied. |
| `guard.permission` | The permission required on the source entity. **Required** when `to` names a non-bare face. |

The loader enforces several rules so that a definition that resolves wrongly
never reaches a reader. A copy into a non-bare face without a `guard.permission`
is refused: an unguarded definition would open the face to anyone who can name
the copy. A copy of an identity-scoped relation is refused, because such an
edge is shared by every face and copying it could duplicate an edge that
confers roles. `guard.when` is accepted by the parser but refused at load with
a message asking you to remove it, because a condition that is written but
never evaluated is worse than none.

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

The web app reads the world from the URL and applies the operator's browsing
default when the URL names none. Two keys in `data-entry.yaml` control this.

Set the browsing default in the `app:` block:

```yaml
app:
  name: "Handbook"
  default_world: published
```

`default_world` names the world a request lands in when it carries no
`?world=`. Without it, browsing shows the bare faces, which are the drafts, and
the published text is the thing you need to know a URL parameter to reach. For
a handbook that is inverted, so the example lands readers in `published` and
editors reach drafts deliberately by selecting `editorial`.

`default_world` is presentation, not policy. It grants nothing: the world's
read grant is re-checked on every request exactly as for an explicit `?world=`,
so pointing it at a world a role may not read yields that world's ordinary
empty result. The server applies it to `curl` and to the browser alike, but
only on read requests and only on routes that can serve a world. An explicit
`?world=default` still reaches the bare faces. Naming an undeclared world here
is a startup error.

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

A create always writes the bare face, and the list is shown in the `published`
world, where a new draft has no face. Without `create_world`, the author would
be redirected to a page that says their new policy has no face here.
`create_world: editorial` opens the form in the editorial world and carries
that world onto the post-create redirect, so the author lands on the draft they
made. It must name a declared world.

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
  `POL-1@published`, and a page showing the draft opens it on `POL-1`. Whether
  a write is offered is the server's `_actions` verdict for that face, so an
  editor looking at an adopted text sees no Edit button (no grant names the
  published face), and the same editor looking at the draft, in whichever
  world, edits the draft. A page showing a face the reader may not write
  carries that face's `messages.read_only` if one is declared, and otherwise
  nothing; the bare face is one click away through the face switcher.
- A **View Published** button, or a menu when there are several faces, lets
  the reader switch to the entity's other faces by address, staying in the
  world they are browsing. It renders on every screen that has faces,
  including the default world.
- A row or card served a **stand-in** (an entity resolved through
  `otherwise: default`, or through a later entry in the chain than the first)
  carries a badge with the world's `messages.stand_in` text, typically
  `{face}`. A first-choice hit shows no badge, and a world that declares no
  text shows none at all.
- An entity that exists but has no face in the world renders its bare face,
  with the world's `messages.absent` if declared. With `on_absent: {redirect:
  <world>}` the app navigates to that world instead.

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
declared faces under `entities.<type>.faces` and its `bare_face`, and the
`default_world` you configured.

Now list the policies a reader sees:

```bash
curl -s "http://localhost:8080/api/v1/policys?world=published"
```

The list contains only policies that have a published face. Omitting the
parameter serves the default world unless `default_world` is configured;
passing `?world=default` explicitly always serves the bare faces.

Read one entity in a world:

```bash
curl -s "http://localhost:8080/api/v1/guides/GUIDE-2?world=site-nl"
```

A single-entity response carries the provenance of the face it served:

```json
"_world": { "name": "site-nl", "face": "en", "via": "chain", "chain_position": 1 }
```

`face` is the declared name of the face that was served. `via` names the
resolution rule: `unscoped` for a type without faces or the default world,
`chain` when a face the world selects exists, and `fallback-default` when the
`otherwise: default` rule substituted the bare face. `chain_position` is the
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
| `/api/v1/_history/{type}/{id}` | Yes. Versioning is per face on PostgreSQL, so the history is the served face's |
| `/api/v1/_next_action` | Yes, as the display world for `visible_worlds` |
| Documents, feeds, analysis, sync, attachments, export, relation sub-resources | No. An explicit `?world=` is refused with `422 world_unsupported` |

Analysis is deliberately unscoped. It reports on the health of the whole graph
a caller may read, and a world that hides a broken draft would make the graph
look clean precisely where it is not.

Search under a world matches the text of the face the world resolves, and an
entity the world excludes has nothing to match. Searching the `published` world
for a word that appears only in a draft returns exactly what searching for a
nonsense word returns.

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

Look for `B10-undeclared-world`, `B11-undeclared-face` and `B12-bare-face-named`
findings, which mark grants that will silently match nothing. The last one is
the spelling warned about in Step 4: `update: [policy@draft]` when `draft` is
the bare face.

Finally, check the stored faces against the schema:

```bash
rela analyze states
```

This reports rows stored under a face no type declares, for example after a
face was renamed or removed from `faces:`, along with face rows whose bare row
is missing and faces stored under a type that does not declare them. It
detects only. To move rows between faces, use the `rename_face` step of the
[data migration system](data-migration.md#renaming-a-content-state).

If your project uses the PostgreSQL backend, each face keeps its own version
history. Editing the draft versions `POL-1@draft`, and invoking `publish`
versions `POL-1@published`. The history page in the web app names the face it
shows, and restoring a version restores that face only.

## What Worlds Do Not Cover Yet

The limits below are deliberate. A surface joins the world-aware set only when
its whole read path has been scoped and tested, so widening the set is a
visible change rather than a forgotten call site.

- The command-line interface has no `--world` flag, and `rela list`, `rela
  show`, and the export commands read the default world. The MCP server and
  Lua scripts read the default world as well.
- Documents, calendar feeds, sync, attachments, exports, and the relation
  sub-resources of an entity refuse a world.
- Restoring a version under a world is refused, like every other write with a
  world.
- A form cannot declare which face it writes. Every form writes the bare face.
- `guard.when` on a copy and `edits:` on a world are parsed but not
  implemented. The first is refused at load, the second is accepted and
  ignored.
- Version history is available on the PostgreSQL backend only.

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
