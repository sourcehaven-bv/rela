# Worlds: one graph, several views

`rela description()`

A **world** answers a question that comes up in every content system: the same
entity has more than one version, and different readers should see different
ones. A policy is drafted, reviewed, then published. A guide exists in English
and Dutch. Neither is a workflow bolted on top — both are the same mechanism.

This handbook is executable. Every claim below is checked against a running
rela when the page is built, so it cannot describe a system that no longer
behaves this way.

## The scenario

Everything below models one small organisation: a company that publishes a
security handbook, in two languages, to an audience outside the team that
writes it.

Three kinds of thing, and the reason each is here:

| | What it is | Why it is in this example |
| --- | --- | --- |
| **Policy** | A rule the organisation commits to — "Access Control", "Remote Working" | Has a **lifecycle**: written, reviewed, then published. Readers must see only the published wording, never the draft. |
| **Guide** | A how-to page — "Getting started" | Has **translations**: English and Dutch, neither more finished than the other. |
| **Control** | A concrete measure a policy relies on — "MFA enforced" | Has **neither**. One version, the same for everyone. It is here to show that worlds are opt-in. |

A policy is an ordinary entity — nothing about faces changes what a type IS:

```rela
typeref{ type = "policy", fields = "required" }
```

Two people use this system, and they want different things from the same data:

- **Edith** edits the handbook. She needs to see drafts — including a policy
  nobody has published yet — because that is the work.
- **Raj** reads it. He must see the published wording and nothing else. Not a
  greyed-out draft, not a "coming soon" placeholder: an unpublished policy
  should be as invisible to him as one that was never written.

That last sentence is the whole problem. Both are looking at the same graph,
and the difference between what they see cannot be a filter someone remembers
to apply — it has to be structural.

## Faces

An entity has one or more **faces**. A face is one content version, addressed
by a coordinate:

```text
POL-1              the default face
POL-1@published    another face of the same entity
```

The type declares which faces exist. A policy declares two, and marks one as
the default:

```rela
faces{ type = "policy" }
```

A guide declares two as well, but along a different axis — language, not
lifecycle:

```rela
faces{ type = "guide" }
```

**Control**, the third type in our scenario, declares none at all. It has
exactly one state, the same in every view. Worlds are opt-in per type: adopting
them for policies costs nothing for controls.

(The ✓ column is `bare_face`, which the *Addressing* section below explains.
It does not affect which face a world picks.)

## Worlds

Once entities have faces, the graph has an extra dimension: a policy is not one
thing, it is a draft *and* a published version. Most of the system — a list, a
detail page, an export — wants a **flat** graph, where each entity is one thing.

A **world is that flattening.** It projects the multi-dimensional graph down to
one face per entity, and it is the only place the choice is made. Nothing
downstream re-decides: a list does not filter drafts out, an export does not
check a status field. They read the projection.

Projecting needs two rules, because two different things can happen.

**Rule one: the chain.** `select:` is an ordered preference. The world walks it
and takes the first face the entity actually has:

```yaml
select: [draft, published]     # prefer the draft; if there is none, published
```

**Rule two: `otherwise:`.** When the entity has *none* of the chain's faces,
the projection has nothing to put in that slot — and there are only two honest
answers. Leave the row out, or substitute the entity's bare-id state.

| World | Chain — first match wins | If none of those exist |
| --- | --- | --- |
| `published` | `published` | **row is dropped** |
| `editorial` | `draft`, else `published` | bare-id state |
| `site-nl` | `nl`, else `en` | bare-id state |

`otherwise:` is mandatory, and this table is why. For **publication** the
answer must be *drop*: a policy with no published face is not merely unstyled
to a reader, it is absent — absence IS the publication bit. For **language** it
must be *substitute*: a guide with no Dutch translation should still be
readable in English. Get these backwards and you either leak drafts to readers
or hide half the site from Dutch ones, so rela refuses to guess a default.

Note the two rules are independent. `editorial` prefers the draft over the
published face for an entity that has both — that is the chain, not the
fallback, and it happens whether or not any face is the bare one.

## What that looks like

Two policies. One is published; one is still a draft.

```rela
create("policy", { id = "POL-1", title = "Access Control", owner = "Security", status = "done" })
face("policy", "POL-1", "published", { title = "Access Control", owner = "Security", status = "done" })

create("policy", { id = "POL-2", title = "Remote Working", owner = "People", status = "doing" })
```

The default world holds every entity at its default face — the whole graph:

```rela
resolution{ type = "policy" }
shows{ type = "policy", exactly = { "POL-1", "POL-2" } }
```

The reader's world holds only what has been published. POL-2 has no published
face, and `otherwise: exclude` means it is not there at all:

```rela
resolution{ type = "policy", world = "published" }
shows{ type = "policy", world = "published", exactly = { "POL-1" }, absent = { "POL-2" } }
```

That `absent` is the whole feature in one line. POL-2 exists, and a reader
cannot see that it exists.

The editorial world declares `otherwise: default`, so the same missing face
resolves to a substitute rather than an absence:

```rela
resolution{ type = "policy", world = "editorial" }
shows{ type = "policy", world = "editorial", exactly = { "POL-1", "POL-2" } }
```

Three diagrams, one set of entities. The difference between them is two lines
of `worlds:` configuration.

### The projection, with the graph underneath

Those three show the OUTCOME. This one shows the choice being made: every face
each entity has, which one the world picked, and the relations they carry.

```rela
create("control", { id = "CTL-9", title = "MFA enforced" })
link("POL-1", "owned-by", "CTL-9")
link("POL-1", "implements", "CTL-9", "published")
resolution{ type = "policy", world = "published" }
```

POL-1 has two faces and the world took the published one; its draft is passed
over, not deleted. POL-2 has only a draft, so under `otherwise: exclude` the
whole row is dropped. The dashed edge is `owned-by`, which is `scope:
identity` — it belongs to the entity and comes along whichever face is chosen.
The solid one is `implements`, `scope: content`: it belongs to a face, so the
projection decides whether it appears at all.

## Addressing: the bare id and the default world

Two mechanisms have been referred to and not explained. Both are about
ADDRESSING — how you name a face, and what you get when you name none.

**`bare_face` names the row a bare id addresses.** Every entity has one row
that `POL-1` resolves to; it exists whether or not you name it. `bare_face:
draft` says the declared name `draft` refers to *that* row, so `POL-1` and
`POL-1@draft` are one row rather than two. Omit it and `draft` becomes a
separate suffixed row while the entity's own row has no name at all — legal,
almost never intended. It sits on the type, so two faces cannot both claim it.

**The default world is what you get with no `?world=`.** It is not a declared
world and applies no projection: every entity resolves to its bare-id row —
the whole graph, one face each, exactly as a system without worlds behaves.
`?world=default` is accepted as an explicit way to ask for the same thing.

These two are why an unpublished policy is still reachable at all. It is absent
from `world=published`, and present in the default world, because that world
projects nothing.

## Writing

Writes address a **face** — they just do not take a `?world=` parameter.

That distinction matters. Writing is fully face-aware — every write names the
face it lands on. What it does not accept is a *world*, which is a routing rule
for reads, not a statement about what a write can reach.

- **Ordinary CRUD** — create, update, delete — addresses the **bare-id row**.
  That is why a new policy is a draft: `bare_face: draft`, so the row a create
  writes is the draft.
- **A copy** addresses **any declared face**. Publishing is a copy from
  `policy@draft` to `policy@published` on the same entity.

What is refused is a world on a write:

```rela
api{
  path = "/api/v1/policys/POL-1?world=published", method = "PATCH",
  as = "editor", body = '{"properties":{"title":"x"}}',
  status = 422, error = "https://rela.dev/errors/world_read_only",
}
```

A refusal rather than a redirect, and the reason is the fallback rule. A world
can answer a read with a substituted face: under `editorial`, reading POL-2
returns its bare-id row because there is no published face to prefer. If a
write could ride that same projection, a read-modify-write through it would
load one face, edit it, and save it over whichever face the caller believed
they were addressing. `DELETE ...?world=published` is worse — it would delete
the entity outright while the caller believed they were unpublishing it.

So a write names its target directly: the bare id, or a copy that declares
which face it writes. Never a projection that might have substituted something.

### Why publishing is an operation, not a field

A copy is declared in the schema, invoked by name, and authorized in its own
right; the operator says which faces it may read and write.

Follow that through and the alternative design disappears. If "published" were
a status field, publishing would be an ordinary update — the same verb as
fixing a typo, reachable by anyone who can edit, and indistinguishable from one
in an audit log. Because it is a copy into a face, an editor cannot publish by
accident: there is no field that means published, only a face, and something
has to put content in it.

## Who may ask for which world

Reading a world is a permission of its own. It is not another entity-type
grant: `published` and `editorial` contain the same entities, so no
create/read/update/delete grant can express the difference between them.

```rela
worlds_matrix{}
```

Only the editor may ask for `editorial`. That is what keeps unpublished work
out of a reader's view — not a property of the drafts themselves, but of who
may request the lens that shows them.

The ordinary capability table is a separate question, and answers it per type:

```rela
roles_matrix{ type = "policy" }
```

Read the reader's row carefully. It is not a plain ✓: the grant is written
`policy@published`, so the reader reads *that face* of a policy and no other.
A bare `policy` grant would read every face, drafts included, which is the
distinction the next section turns into an assertion.

Writing is the narrower grant. An editor may change a policy; neither the
reader nor the translator may, though the translator owns guides:

```rela
permits{ who = "edith@example.com", op = "update", type = "policy" }
refuses{ who = "raj@example.com",   op = "update", type = "policy" }
refuses{ who = "tine@example.com",  op = "update", type = "policy" }
permits{ who = "tine@example.com",  op = "update", type = "guide" }
```

### The grant that keeps drafts unpublished

Being unable to *write* a draft is not what keeps it unpublished — a reader who
could not write it but could still fetch it would have read every unreleased
policy. What conceals a draft is the READ grant, and specifically the fact that
it names a face: `read: [policy@published]` rather than `read: [policy]`.

That distinction is invisible in the yaml until you test it, so the manual
tests it. The reader may fetch the published face:

```rela
reads{ who = "raj@example.com", type = "policy", id = "POL-1", face = "published" }
```

And cannot reach the draft behind it, even by naming it directly:

```rela
hidden{ who = "raj@example.com", type = "policy", id = "POL-1" }
```

The editor, holding a bare `policy` grant, reaches the same draft:

```rela
reads{ who = "edith@example.com", type = "policy", id = "POL-1" }
```

These three claims are what a bare `read: [policy]` would break — the reader
would still be unable to write a draft, every `refuses{}` above would still
pass, and the drafts would be readable by anyone. That is why the read side is
asserted separately rather than inferred from the write side.

#### Withholding a face means not mentioning it either

Refusing to serve the draft is only half of concealing it. The response the
reader *is* allowed to fetch carries a `_faces` list — the "other versions of
this page" menu — and naming the draft there would disclose that an unpublished
version exists, which is the very thing the grant withholds.

So the same request answers differently for the two roles. The reader is served
the published policy and is told about no other face:

```rela
api{
  path = "/api/v1/policys/POL-1?world=published", as = "reader", status = 200,
  has = { "Access Control" }, absent = { "Draft" },
}
```

The editor, whose grant covers every face, is offered the draft on the same URL:

```rela
api{
  path = "/api/v1/policys/POL-1?world=published", as = "editor", status = 200,
  has = { "Draft" },
}
```

The pair is the point. Either claim alone could pass for a boring reason — a
misspelled face name would be `absent` from every response — so the manual
asserts that the string is present for one principal and missing for the other.

The same reasoning covers version history, which is a second way to ask for a
face's content. A reader who cannot fetch the draft cannot read its timeline
either:

```rela
api{ path = "/api/v1/_history/policy/POL-1?world=default", as = "reader", status = 404 }
```

That request names the default world explicitly, because this project sets
`default_world: published`: without the parameter the reader is asking for the
published face's history, which they may read. The world is what selects the
face, so the assertion has to name it to be about the draft at all.

## Relations belong to a face, or to the entity

An edge can be scoped two ways, and the choice is per relation type:

```rela
relations{ type = "policy" }
```

`implements` is `scope: content` — it belongs to the FACE, so a draft may
implement a different control set than the published face does. `owned-by` is
`scope: identity` — one owner for the entity, whatever face you are reading.

```rela
create("control", { id = "CTL-1", title = "MFA enforced" })
create("control", { id = "CTL-2", title = "Quarterly access review" })
link("POL-1", "owned-by", "CTL-1")
```

## Guides: the same mechanism, no lifecycle

Nothing above was about approval. Swap the axis and the machinery is identical:

```rela
create("guide", { id = "GUIDE-1", title = "Getting started" })
face("guide", "GUIDE-1", "nl", { title = "Aan de slag" })

create("guide", { id = "GUIDE-2", title = "Incident response" })
```

Both guides appear in the Dutch world. GUIDE-1 resolves to its Dutch face;
GUIDE-2 has no translation, and because `site-nl` declares `otherwise:
default` it falls back to English rather than disappearing:

```rela
resolution{ type = "guide", world = "site-nl" }
shows{ type = "guide", world = "site-nl", exactly = { "GUIDE-1", "GUIDE-2" } }
```

Both appear to a reader too, because `published` overrides guides to `en`:

```rela
shows{ type = "guide", world = "published", exactly = { "GUIDE-1", "GUIDE-2" } }
```

Compare that with policies, where the same world excludes. One schema, two
answers, because the operator said which one each type needed.

### A relation that crosses faces

Fallback is not only about whether an entity appears — it also decides what a
*relation* lands on. `see-also` is `scope: content`, so the edge belongs to one
face: point the **Dutch** face at the untranslated guide by naming that face as
the edge's tail.

```rela
link("GUIDE-1", "see-also", "GUIDE-2", "nl")
```

Two separate lookups happen when you read GUIDE-1 in `site-nl`, and keeping them
apart is what makes the result honest.

**The edge is looked up on the face you are reading.** Nothing falls back here.
The Dutch face has this `see-also`, so you see it; had you only linked the
English face, the Dutch reader would see no link at all rather than borrowing
one. A content-scoped edge belongs to its face, and inventing a fallback would
contradict that.

**The target then resolves through the world on its own.** GUIDE-2 has no Dutch
face, so the chain falls through to English — and that is the one you reach:

```rela
resolution{ type = "guide", world = "site-nl" }
```

The detail view badges that, so a reader can see when a link leads somewhere
other than the face they are reading — there is a screenshot of it below, under
The screens.

This is why neighbours resolve through the world rather than being read at
their bare id. An unlabelled link would silently hand a Dutch reader an English
page; a link read at the bare id would ignore the translation even when one
exists. Resolving the neighbour and naming its face is what makes the fallback
honest instead of invisible.

## What the operator configures

Everything above is declared, not coded. Three files:

- **`schema.yaml`** — which faces a type has, and which worlds select them.
- **`data-entry.yaml`** — which screens exist, and which world each opens in.
- **`acl.yaml`** — who may read which world.

The `worlds:` block is the whole feature. A world names a `select:` chain and
a mandatory `otherwise:`; a per-type `overrides:` handles the case where one
type's chain differs, as `guide` does under `published`.

```rela
graph{ from = "policy", depth = 2 }
```

## The screens

The same configuration drives every screen. What follows is the running app,
captured while this page was built.

### The list

A list renders in a world. Under `published` a reader sees only what has been
published — POL-2 is not greyed out or marked draft, it is simply not there:

```rela
screenshot{
  view = "list", list = "policies", world = "published", as = "reader",
  out = "list-published.png",
  alt = "The policies list in the published world, showing only POL-1",
}
```

That figure is checked, not just captured. `page{}` asserts against the same
rendered screen the screenshot above photographs — so the caption cannot drift
from the picture:

```rela
page{
  view = "list", list = "policies", world = "published", as = "reader",
  menu_has = { "Policies", "Pipeline" },
  region = "list", has = { "Access Control" }, absent = { "Remote Working" },
}
```

That `absent` is the publication bit, made in the one place a screenshot cannot
make it: not "POL-2 is styled as a draft", but "the string *Remote Working* does
not occur in the reader's table at all".

The same list, same data, editorial world:

```rela
screenshot{
  view = "list", list = "policies", world = "editorial", as = "editor",
  out = "list-editorial.png",
  alt = "The policies list in the editorial world, showing both policies",
}
```

An editor's table holds both, which is the same claim from the other side:

```rela
page{
  view = "list", list = "policies", world = "editorial", as = "editor",
  region = "list", has = { "Access Control", "Remote Working" },
}
```

#### A row that is a stand-in

Presence is only half of what a world decides. The other half is *which face*
each row is showing — and the chain can substitute one silently, because a
fallback row is byte-identical to a first-choice hit: same id, same title, same
cells.

Policies cannot show it. `draft` is their `bare_face`, so every policy has a
draft row by construction — a face row cannot exist without the bare one it
hangs off — and `editorial`'s chain `[draft, published]` therefore always
matches at its first choice. The substitution is real but structurally
unreachable on this axis. That is why the two policy tables above carry no
badges at all: every row in them is the face its world asked for, and the badge
only marks the rows that are not.

Guides can. `site-nl` selects `[nl, en]` where `en` is the bare face, so a guide
with no translation falls through to its English row: GUIDE-1 is Dutch, GUIDE-2
is a stand-in. Same list, one world, two different kinds of row:

```rela
screenshot{
  view = "list", list = "guides", world = "site-nl", as = "reader",
  out = "list-guides-nl.png",
  alt = "The guides list in site-nl: GUIDE-2 badged English as a fallback, GUIDE-1 unbadged",
}
```

That figure is the clearest picture of the rule the badge follows. One row is
marked and one is not, and the marked one is the stand-in: "Incident response"
carries an "English" badge because `site-nl` asked for Dutch and could not get it.
"Aan de slag" *is* the Dutch face, so it is left alone.

The badge is an EXCEPTION marker, not a provenance label on every row. Under
`site-nl` every row has a face and the world could label all of them — but a
badge on every row is noise, and noise trains a reader to stop looking. Marking
only the surprise is what keeps the mark worth reading.

What the badge *says* is the operator's. The web app has no words of its own
for a stand-in — "fallback", "face", "default" are its storage vocabulary, not
a reader's — so `site-nl` declares `messages: { stand_in: '{face}' }` in
`schema.yaml`, and `{face}` is substituted with the served face's label. A
world that declares nothing marks nothing; whether a row is a stand-in is
still the server's answer, but the mark waits for words.

Two claims, and the second is the one a screenshot cannot make. First, both
rows are present — the fallback substitutes rather than excluding, which is what
`otherwise: default` buys:

```rela
page{
  view = "list", list = "guides", world = "site-nl", as = "reader",
  region = "list", has = { "Aan de slag", "Incident response" },
}
```

Second, the badge names the substituted face and *only* that one. The `absent`
half is the stronger claim: not merely that "English" appears somewhere, but
that "Nederlands" appears nowhere in the badge region — the translated row is
unmarked:

```rela
page{
  view = "list", list = "guides", world = "site-nl", as = "reader",
  region = "badge", has = { "English" }, absent = { "Nederlands" },
}
```

The "English" half is load-bearing beyond the badge. `en` is this type's
`bare_face`, so it is stored at the *bare* row and comes back from the API as
the empty coordinate; a `{face}` handed that empty string would print nothing.
Reading "English" here asserts that the coordinate resolves back through the
same `bare_face:` declaration the world chain was compiled with, to the label
the operator gave that face — never the coordinate itself, which is storage
vocabulary.

### The detail view

A detail page states which world it is showing and which face it resolved to —
provenance a reader can act on, rather than a page that silently differs:

```rela
screenshot{
  view = "entity", type = "policy", entity = "POL-1", world = "published",
  as = "reader", out = "detail-published.png",
  alt = "POL-1 detail page in the published world",
}
```

Provenance matters most where a page mixes faces. Read the Dutch guide in
`site-nl` and its own face is Dutch, but the `see-also` link resolves to a guide
that has no Dutch face — so the neighbour is served in English. Neighbours
resolve independently, so one section can mix faces, and the badge falls on
exactly the ones that came back as stand-ins:

```rela
screenshot{
  view = "entity", type = "guide", entity = "GUIDE-1", world = "site-nl",
  as = "reader", out = "detail-guide-nl.png",
  alt = "GUIDE-1 in Dutch, with a badge on its see-also link to GUIDE-2",
}
```

The badge names the **face**, not the world. Those are easy to confuse and the
difference is the whole point: "site-nl" says which world you asked for, which
you already know; "English" says the link leads somewhere English, which you
did not.

Note that `en` is the guide type's `bare_face`, so it is stored at the bare row
with no coordinate of its own. The badge still prints "English", because the
API resolves the stored row back through the same `bare_face:` declaration the
world chain was compiled with, and the label is the operator's own name for
that face:

```rela
page{
  view = "entity", type = "guide", entity = "GUIDE-1", world = "site-nl",
  as = "reader",
  region = "badge", has = { "English" }, absent = { "Nederlands" },
}
```

The `absent` half says the same thing the guides list says: GUIDE-1's own Dutch
face is not badged. The page is Dutch, which is what the reader asked for; the
one thing worth marking is the link that leaves it.

#### What that page does NOT announce

Look at the top of that figure and compare it with the published one above it.
The published page opens with a line saying so; the Dutch page opens with no
announcement at all. Both are correct, and the difference is one optional line
of operator config.

`banner:` on a world in `schema.yaml` is the operator's announcement, and it is
optional. Unset means no announcement — which is why `site-nl` declares none.
Telling someone who asked for the Dutch site that they are reading Dutch says
nothing they did not already know. `published` and `editorial` do carry banners,
because "this is what readers see" and "drafts included" are genuinely not
obvious from the page in front of you: two policies can look identical and
differ only in whether the world admitted the unpublished one.

What the operator cannot switch off is the way back. A reader may have
arrived in a world from a link, and a suppressible exit would leave them with
no way out of it; the face switcher is the honesty of the projection, and
configuration does not reach it. The words around it are a different matter.
A page that reached a face the reader may not write says so only in the
operator's sentence for that face (`faces.<name>.messages.read_only`), and a
reader who can act on the page needs no explanation, any more than one
without update permission does. "Face", "world" and "bare" are rela's storage
vocabulary; a reader never chose those words, so the web app never says them.

### The create form

A new policy is a draft, so the create button opens the form in the editorial
world — otherwise the author would be returned to a world their new entity is
not in:

```rela
screenshot{
  view = "create", type = "policy", world = "editorial", as = "editor",
  out = "create-policy.png",
  alt = "The new-policy form",
}
```

### The edit form

The edit form is reached from a world, and the screen keeps showing you which
one — but the write itself addresses a face, exactly as the Writing section
described. The world you browsed in decided *which face you are looking at*; it
does not travel with the save:

```rela
screenshot{
  view = "form", type = "policy", entity = "POL-1", world = "editorial",
  as = "editor", out = "edit-policy.png",
  alt = "The policy edit form",
}
```

### The kanban

A board is a projection too. Same policies, same `status` column property —
what differs is which face each card is showing, and whether it is there at all.

Workflow status and face are independent: the column a card sits in is a
property of the content, while the face is which version of it you are reading.
A policy can be `done` and still unpublished.

A third policy makes the board worth looking at — published, and carrying a
draft that has moved on since:

```rela
create("policy", { id = "POL-3", title = "Data Retention", owner = "Legal", status = "done" })
face("policy", "POL-3", "published", { title = "Data Retention", owner = "Legal", status = "done" })
```

All three reach the editorial board, each at its draft face, because that is
`editorial`'s first choice and every policy has one:

```rela
resolution{ type = "policy", world = "editorial" }
shows{ type = "policy", world = "editorial", exactly = { "POL-1", "POL-2", "POL-3" } }
```

```rela
screenshot{
  view = "kanban", list = "pipeline", world = "editorial", as = "editor",
  out = "kanban-editorial.png",
  alt = "The editorial pipeline board with all three policies at their draft faces",
}
```

All three policies reach the editorial board, and none of them reaches the
reader's — the projection itself is right:

```rela
page{
  view = "kanban", list = "pipeline", world = "editorial", as = "editor",
  has_card = { "Data Retention", "Remote Working", "Access Control" },
}
```

Notice that no card on this board carries a badge. That is not the board
skipping a label it owes you — it is the same rule the lists follow. `editorial`
selects `[draft, published]`, `draft` is policy's `bare_face`, and a face row
cannot exist without the bare one it hangs off, so every policy matches at the
chain's first choice. There are no stand-ins on this board, so there is nothing
to mark, and a clean board is the honest rendering of that.

#### A board that does have one

To see the other case you need an axis where the world's first choice can
actually be missing. A **procedure** is the handbook's operational runbook —
written centrally in English, localised per site where the local team needs it
in their own words. Same face layout as a guide (`bare_face: en`, plus `nl`),
but a procedure also carries a readiness state, and that is what gives it a
board:

```rela
create("procedure", { id = "PRC-1", title = "Restore from backup", readiness = "drilled" })
face("procedure", "PRC-1", "nl", { title = "Herstellen vanaf back-up", readiness = "drilled" })

create("procedure", { id = "PRC-2", title = "Revoke a leaver's access", readiness = "drilled" })
```

PRC-1 is localised; PRC-2 is not. Under `site-nl` the chain is `[nl, en]`, so
PRC-1 resolves at its first choice and PRC-2 falls through to English — a
genuine substitute, on a board:

```rela
resolution{ type = "procedure", world = "site-nl" }
```

```rela
screenshot{
  view = "kanban", list = "readiness", world = "site-nl", as = "reader",
  out = "kanban-readiness-nl.png",
  alt = "The procedure readiness board in site-nl: the leaver-access card badged English, the Dutch backup card unbadged",
}
```

Both cards sit in the same column and are the same shape. One is badged and one
is not, and that mark is the whole difference between reading a localised
procedure and reading an English one under a Dutch world — a difference that
matters rather more on a runbook than on a handbook page:

```rela
page{
  view = "kanban", list = "readiness", world = "site-nl", as = "reader",
  region = "badge", has = { "English" }, absent = { "Nederlands" },
}
```

Readiness is worth a second look too, because it is a third axis and not a
disguised face. A procedure can be `drilled` in English and have no Dutch face
at all; a freshly translated one starts back at `untested` even though the
English procedure was signed off long ago. Readiness travels with the face, the
localisation is a different question, and the world decides which of the two
states you are reading — the same independence `status` has from publication on
the policy board, arrived at from the other side.

### The dashboard

Counts and breakdowns are computed from the projection, not from the whole
store — so a reader's dashboard reports the published handbook, and an
editor's reports the work in progress:

```rela
screenshot{
  view = "dashboard", world = "published", as = "reader",
  out = "dashboard-published.png",
  alt = "The dashboard in the published world",
}
```

### Next actions

The dashboard also carries a suggestion: one advisory hint about what to do
next, drawn from sources the operator declares. A world touches it twice, and
the two are separate keys because they answer separate questions.

**`source_world:` decides what is found.** It is the world a source's candidate
query runs in. **`visible_worlds:` decides where it is shown** — an allow list
of worlds the suggestion may be displayed in. Unset, it matches every world.

Take the obvious pairing first, and watch it fail. A suggestion is nearly
always about *unfinished* work, and unfinished work is exactly what `published`
leaves out. A source that simply queried whatever world the reader was standing
in would be reliably empty in the world most people browse. So the query world
is declared per source and never taken from the request:

```yaml
finished-draft:
  source_world: editorial       # look for drafts, wherever the reader stands
  query: "type:policy prop:status=done"
```

`visible_worlds` is unset there, so the hint follows the reader everywhere.
Here it is on the reader's own dashboard — a world in which the draft it names
is not itself visible:

```rela
screenshot{
  view = "dashboard", world = "published", as = "editor",
  out = "next-action-published.png",
  alt = "The published dashboard carrying a suggestion about a draft policy",
}
```

That is the combination worth learning. The suggestion was found in
`editorial` and displayed in `published`, and neither decision leaked into the
other:

```rela
page{
  view = "dashboard", world = "published", as = "editor",
  region = "next-action", has = { "is marked done" },
}
```

The other axis restricts display instead. A procedure with no Dutch face falls
back to English, and "the Dutch site is still reading the English runbook" is
upkeep for whoever works on that site — and noise to everyone else. So the
source is displayed only there:

```yaml
translate-procedure:
  source_world: site-nl
  visible_worlds: [site-nl]     # relevant here, noise everywhere else
```

Under `site-nl` it appears:

```rela
screenshot{
  view = "dashboard", world = "site-nl", as = "translator",
  out = "next-action-site-nl.png",
  alt = "The site-nl dashboard carrying the translation suggestion",
}
```

```rela
page{
  view = "dashboard", world = "site-nl", as = "translator",
  region = "next-action", has = { "Dutch site" },
}
```

And under `published` it does not — the stronger half of the claim, and the one
a screenshot cannot make, because "no suggestion" and "a different suggestion"
both look like a page:

```rela
page{
  view = "dashboard", world = "published", as = "translator",
  region = "main", absent = { "Dutch site" },
}
```

Sources are grouped into ordered bands and evaluation stops at the first band
that yields a candidate, so the two figures above differ in prominence as well
as in text: `finished-draft` sits in a banner band, `translate-procedure` in a
quieter notice one.

**`visible_worlds` is presentational, not a confidentiality boundary.** A
suggestion omitted from a world is omitted because it would be noise there, not
because its content is secret. Candidates reach the engine through the caller's
read gate either way, so a source can never surface an entity its reader may
not read, and everything one names stays reachable through the ordinary read
API. `?world=` on the request selects the display world *only*; it scopes no
read, so a caller cannot aim an operator's rule at a world the operator never
named. Never reach for this list to hide something.

### History

Content versioning is per-face: a policy's draft and its published face have
genuinely different histories, so the page names which face it is showing.

Version history is a capability of the **postgres backend** — `HistoryReader`
is implemented by `pgstore` and by nothing else — so the figures below come
from a build of this manual against PostgreSQL (`just docs-visual-postgres`).
On the filesystem backend the same page reports that history is unavailable,
which is a truthful answer rather than an empty timeline.

Here POL-4 is drafted and then revised. Each `edit` is a real write through the
entitymanager — authorized, attributed, and captured as a version — rather than
a fixture rewritten in place:

```rela
create("policy", { id = "POL-4", title = "Device Encryption", owner = "IT", status = "to-do" })
edit("POL-4", { status = "doing" })
edit("POL-4", { title = "Device Encryption Standard", owner = "Security" })
```

A version is **not** one row per write. Capture is a debounced reconciliation
sweep that snapshots an entity once it has *settled*, so a burst of edits like
the one above becomes a single version holding the state they add up to. That is
deliberate: history is meant to record what a document became, not to replay
every keystroke on the way there. Two edits a week apart are two versions; two a
second apart are one.

The timeline shows that settled state against the face the world resolved:

```rela
screenshot{
  view = "history", type = "policy", entity = "POL-4", world = "editorial",
  as = "editor", out = "history-policy.png",
  -- Versions arrive from a background sweep, so the capture WAITS for the row
  -- this section claims rather than photographing an empty timeline.
  await_versions = 1,
  alt = "Version history for POL-4, showing the captured version and its author",
}
```

The subtitle names the face, and that is the whole point of the per-face claim.
Under `editorial` a policy resolves to its draft, so this is the DRAFT's
history — the published face has its own.

#### History across a copy

Publishing is a **copy** from `policy@draft` to `policy@published`, declared in
the schema and invoked by name:

```rela
api{
  path = "/api/v1/_copies/publish", method = "POST", as = "editor",
  body = '{"source_id":"POL-4"}',
  status = 200,
}
```

That copy writes the *published* face. Because `entity_versions` is keyed by
`(entity_id, face, vseq)`, the published face accumulates its own lineage: the
two faces share an id and share nothing else in their history. Asking for the
history of one never returns the other's — which is why the page names the face
it is showing rather than leaving the reader to assume.

Reading the published face's history is the same request with a different
world, and the server answers with the face it resolved:

```rela
api{
  path = "/api/v1/_history/policy/POL-4?world=published", as = "editor",
  status = 200,
}
```

And it can be shown, not merely asserted. This figure is the published face's
own timeline — the version the copy above just wrote:

```rela
screenshot{
  view = "history", type = "policy", entity = "POL-4", world = "published",
  as = "editor", out = "history-published.png",
  -- The copy is a real write through the API, and versions arrive from a
  -- debounced sweep — so wait for the row rather than photographing an empty
  -- timeline and captioning it as a history.
  await_versions = 1,
  alt = "Version history for the published face of POL-4, written by the publish copy",
}
```

Note what that figure depends on. The copy was made by an `api{}` island a few
paragraphs up, and the screenshot is taken against the *same* project — so the
picture shows the result of a write this document performed, not of a fixture
seeded to look like one. Islands run top to bottom and share one store, which
is what makes a manual able to photograph its own consequences.

The practical consequence is that restoring a version restores *that face*.
Rolling the published face back to what it said last week does not touch the
draft an editor is still working on, and rolling the draft back does not
un-publish anything.

### Analyze

Analysis deliberately stands outside the projection. It refuses a world rather
than answering one:

```rela
api{
  path = "/api/v1/_analyze?world=published", as = "editor",
  status = 422, error = "https://rela.dev/errors/world_unsupported",
}
```

That is a considered position rather than a missing feature: analysis is about
the graph's *health*, not about its presentation.

Analysis looks for orphans, cardinality violations and failed validations. Those
are questions about whether the underlying data is sound — and a world hides
things by design. A policy excluded from `published` is invisible there, so
analysing that world would report the graph as clean precisely where a draft is
broken. Worse, `otherwise: exclude` would make a genuine cardinality violation
disappear rather than surface. Analysis therefore runs over everything the
principal may read, and reports the real state.

It stays unscoped, and says so with an explicit 422 rather than silently
ignoring the parameter — a refusal an operator can act on, instead of a number
that quietly meant something else.

```rela
screenshot{
  view = "analyze", as = "editor",
  out = "analyze.png",
  alt = "The analysis screen, which reports on the whole graph",
}
```

### Search

Search is a projection too, and this is the case where it matters most. A
reader typing into the search box is asking "what is there?" — and if the index
answered from outside the world, that box would be a way to read the titles of
things the world exists to hide.

So a world **is** the search scope. Searching `published` means searching each
entity's published face: the text that decides a hit is the text the reader
will be shown, and an entity with no face in the world has nothing to match at
all.

Both halves are load-bearing, and the second is the one to be careful about.
POL-2 has only a draft, so under `otherwise: exclude` it is absent from the
reader's world — including from its search. Searching a reader's world for a
word that appears **only** in that draft must be indistinguishable from
searching for a word that appears nowhere:

```rela
api{
  path = "/api/v1/policys?q=Remote&world=published", as = "reader",
  status = 200,
  identical_to = { path = "/api/v1/policys?q=Zzzzz&world=published", as = "reader" },
}
```

"Remote" is the title of POL-2, which the reader may not see; "Zzzzz" is in no
entity at all. The two answers being byte-identical is the claim — a status
assertion alone could not make it, because "found nothing" and "found the
draft" are both a 200.

Edith gets the other answer from the same query, because her world holds POL-2:

```rela
shows{ type = "policy", world = "editorial", contains = { "POL-2" } }
```

Note what is *not* happening here. Nothing filters drafts out of the results
after the fact, and there is no per-world index to keep in step. The world
resolves each entity to one face first, and only that face is matched — so the
same rule that decides which policies appear in a list decides which ones a
search can find, and neither can drift from the other:

```rela
screenshot{
  view = "search", q = "Access", as = "editor",
  out = "search-default.png",
  alt = "Searching in the default world",
}
```
