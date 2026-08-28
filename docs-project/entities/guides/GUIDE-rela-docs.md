---
id: GUIDE-rela-docs
type: guide
title: "Generated documentation: the rela docs language"
status: published
order: 22
audience: intermediate
summary: "Author a deployment manual in Markdown with embedded Lua islands that pull reference fragments (field tables, enum meanings, mermaid lifecycles, relation graphs, role matrices) straight from the schema."
---

`rela-docs build` renders a **manual** — ordinary Markdown you write by
hand — resolving embedded **Lua islands** that pull mechanical reference
fragments from the deployment's schema (and a small in-memory graph the
manual seeds). Prose stays prose; the field tables, enum meanings, state
diagrams, relation graphs, and role matrices are generated so they can
never drift from `schema.yaml` / `acl.yaml`.

This is the *reference* half of a manual, generated; the *explanation*
and *how-to* halves stay hand-written around it.

## The two island forms

A manual is Markdown by default. You escape into Lua two ways:

- **Statement island** — a fenced ` ```rela ` block. Its Lua runs for
  side effects; emit calls (`h2`, `md`, and the resolvers below) append
  Markdown at that position. Use it for tables, diagrams, and loops.
- **Echo island** — an inline `` `rela <expr>` `` code span. The
  expression is evaluated and its string value substituted mid-sentence.

A code span or fenced block that does **not** start with the `rela`
marker is left untouched, so you can show the doc language literally in
a `` ```markdown `` sample.

```markdown
# Risicobeheer

`rela description()`

Elk *risico* legt de onderstaande gegevens vast:

​```rela
typeref{ type = "risico", fields = "required" }
​```

Er zijn `rela count{ type = "risico" }` risico's geregistreerd.
```

## Resolvers

Each resolver is a function you call from an island. Names are also
available without the `doc.` prefix.

| Function | Emits |
| --- | --- |
| `typeref{type, fields="required"\|"all"}` | a field table (name, type, required), in definition order |
| `values{type, field}` | an enum's values, the default marked, plus per-value meaning when the custom type declares `descriptions:` |
| `relations{type}` | the type's outgoing relations as a list |
| `graph{from, depth, direction, exclude\|only}` | a mermaid flow graph (see below) |
| `lifecycle{type, field}` | a mermaid `stateDiagram-v2` for a state-machine field (fails loud on a flat enum — use `values{}` there) |
| `entity{id, fields}` | one **seeded** entity's fields |
| `count{type}` | the number of seeded entities of a type (echo-friendly) |
| `roles_matrix{type}` | a role × verb capability table from `acl.yaml` (omit `type` for every type) |
| `description()` | the metamodel's top-level `description:` (echo-friendly) |
| `shows{type, contains\|absent\|exactly}` | **asserts** which entities of a type exist (emits nothing) |
| `refuses{who, op, type, because, unassigned}` / `permits{...}` | **asserts** an authorization outcome (emits nothing) |
| `api{path, status, error, as, identical_to}` | **asserts** an API contract against a real server (emits nothing) |
| `h1/h2/h3(text)`, `md(text)` | structural Markdown emitted from Lua |

### Relation graphs

`graph{}` has two grains, chosen by `from`:

- `from = "<entity type>"` draws the **schema** neighbourhood — which
  types connect to which — from the metamodel.
- `from = "<seeded id>"` traverses the **seeded** instance graph.

`depth` bounds the hops (default 2, capped at 5). `direction` is `"out"`
(default), `"in"`, or `"both"`. `exclude = {"rel_a", "rel_b"}` prunes
noisy relation types — the thing that turns a hairball into a readable
diagram; `only = {...}` is the allowlist inverse (the two are mutually
exclusive).

```markdown
​```rela
graph{ from = "verwerking", depth = 2, exclude = { "spawnt", "gaat_over" } }
​```
```

## Seeding example data

Instance resolvers (`entity`, `count`, `graph{from=<id>}`) read a fresh,
**in-memory** graph that exists only for the build — never your real
entities. The manual populates it with `create` and `link`, the same
verbs a Lua script uses:

```markdown
​```rela
local r = create("risico", { titel = "Onbevoegde toegang", kans = 2, impact = 3 })
local m = create("maatregel", { titel = "MFA afdwingen" })
link(r, "wordt_gemitigeerd_door", m)

entity{ id = r.id, fields = { "titel", "kans", "impact" } }
graph{ from = r.id, depth = 1 }
​```
```

Seed writes go **straight to the in-memory store** — there is no
validation, no state-machine gate, and no ACL. A fixture is exactly what
you write, so `create("risico", { status = "done" })` is fine even if
`done` is not a legal entry state. This keeps fixtures honest and
self-contained; it never touches the project on disk.

## Assertions

A manual can *check* the claims its prose makes. Assertion islands emit no
output — they either pass, or they fail the build with the mismatch:

```rela
create("risico", { titel = "Leak", kans = 3, impact = 4, status = "todo" })

shows{ type = "risico", exactly = { "risico-1" } }
refuses{ who = "bob@example.com", op = "update", type = "risico" }
```

This is what keeps a handbook from outliving the system it describes. If
someone widens `viewer` in `acl.yaml`, the `refuses{}` above stops holding and
the manual refuses to build, naming the rule that fired:

```text
manual:80: resolve: refuses{who="bob@example.com", op="update", type="risico"} failed
  claimed: refused
  actual:  PERMITTED
  rule:    role-grant/viewer
```

**Every argument is optional except the target.** A paragraph about visibility
says nothing about buttons, so `shows{}` asserts only the claims you give it.
But **a call that asserts nothing is an error** — `shows{type="risico"}` looks
like a test and checks nothing, which is the one failure mode worse than having
no test at all.

Prefer `exactly` over `contains` for a set you fully control: `contains` cannot
see an over-inclusive result, and over-inclusion (a leaked row, a duplicate) is
usually the interesting bug. `exactly = {}` is a real claim — "this type has no
entities" — not an empty one.

`who` must name a principal in `acl.yaml`'s `assignments`. An unknown one is
refused, because a principal with no assignment has no grants and is therefore
denied *by construction* — so a `refuses{}` with a misspelled `who` would pass
forever, unable to ever fail. When the missing assignment IS the point ("there
is no self-service sign-up"), say `unassigned = true`; without it, an intended
claim and a typo look identical to a reviewer.

`refuses{}` / `permits{}` evaluate through the **same authorization path the
write path uses**, not by re-reading `acl.yaml`. Answering from the policy file
would only prove that the manual and the file agree, which they would even if
the gate were never consulted. This is the deliberate exception to the
"seed writes bypass ACL" rule above: seeding is a fixture, an assertion is a
claim about real behaviour. Add `because = "..."` to also pin *why* a decision
came out that way — a deny arriving from an unintended rule is a green check
over a real regression.

### API assertions

`api{}` issues a real request against the documented project's API, served over
a throwaway temp copy seeded with the manual's own fixtures:

```rela
create("ticket", { id = "TKT-1", title = "Login 500s", status = "open" })

api{ path = "/api/v1/tickets/TKT-1", as = "editor", status = 200 }
api{ path = "/api/v1/tickets/NOPE", as = "editor", error = "https://rela.dev/errors/not_found" }
```

`error=` claims the **machine-readable code**, not the prose title: a message
gets reworded, the code is the contract. Asserting the message gives a check
that fails on a copy edit and passes on a real behaviour change.

`as=` names a role from `acl.yaml`, so an ACL claim can be stated as the
response a real caller gets.

Unlike `screenshot{}`, `api{}` needs no browser and no built frontend — it only
reaches the `/api/v1` handlers, so it can gate CI unconditionally. Like
`screenshot{}` it is unavailable on the `postgres` build, where seeding a
"throwaway" project would write into the live database.

#### `identical_to` — the existence-oracle property

Some properties are not about one response but about two being the **same**.
"A denied read is indistinguishable from a missing entity" cannot be stated by
asserting a status, because the whole claim is that two requests answer alike:

```rela
api{
  path = "/api/v1/tickets/HIDDEN", as = "viewer", status = 404,
  identical_to = { path = "/api/v1/tickets/NO-SUCH-THING", as = "viewer" },
}
```

The comparison covers status, body, and the response headers that can by
themselves disclose existence (`ETag`, `Last-Modified` — a denied GET that
emits an ETag lets a replayed `If-None-Match` return 304, confirming the entity
exists). It **excludes** the problem-details `instance` member, which echoes the requested URL: two requests to different
urls necessarily differ there, so including it would make the check fail on
every pair — a check that never passes is not checking anything. `instance`
reflects only what the caller already typed, so it leaks nothing; every other
field must match exactly.

The exclusion is minimal and route-specific: on `/api/v1/{plural}/{id}` the two
404s differ only in `instance`. Some other routes interpolate the request into
the problem `title` (`entity %q not found`), so `identical_to` will fail there —
in the safe direction, but it means the verb is meaningful on the entity routes
and needs checking before use elsewhere.

## Building

Manuals are built with the **`rela-docs`** binary — a separate tool from
`rela`. It is split out because `screenshot{}` (below) drives a headless
browser, whose dependency the everyday `rela` binary never needs to carry.

```bash
rela-docs build manual.md            # resolved Markdown to stdout
rela-docs build manual.md --out site/manual.md
rela-docs build manual.md --strict   # fail if any island resolves to nothing
```

The metamodel and `acl.yaml` come from the project (`--project` or the
current directory). The output is Markdown — pipe it through `pandoc`
for PDF, or commit it beside hand-written chapters.

**Fail-loud.** An island referencing an unknown type or field, a Lua
error, or (under `--strict`) an empty resolve stops the build with the
offending **manual** line — never a silently missing figure. Because
islands are real Lua, you also get loops and conditionals for free:

```markdown
​```rela
for _, t in ipairs({ "risico", "maatregel", "verwerking" }) do
  h3("Velden — " .. t)
  typeref{ type = t }
end
​```
```

## Screenshots

The `screenshot{}` island captures a **live screenshot of the data-entry
form** for a seeded entity, annotated with arrows, and embeds it as a
Markdown image. It stands up the data-entry app over a throwaway copy of
your project (seeded with the manual's `create`/`link` entities) and
drives a headless Chrome to render the real form.

```markdown
​```rela
local t = create("ticket", { id = "DEMO-1", title = "Login page 500s",
                             status = "in-progress", priority = "high",
                             reporter = "demo@example.com" })
screenshot{
  view = "form", type = "ticket", entity = t.id,
  arrows = {
    { at = "status",   text = "the lifecycle state" },
    { at = "priority", text = "triage priority" },
  },
  out = "ticket-form.png",
  alt = "The ticket edit form",
}
​```
```

Arguments:

| Arg | Meaning |
| --- | --- |
| `view` | `"form"` — the edit form (the only view supported today; the default) |
| `type`, `entity` | the entity type and the id of a **seeded** entity to render |
| `form` | the `data-entry.yaml` form id (default `edit_<type>`) |
| `as` | the ACL **role** to render as (a principal assigned that role); default picks a role that can edit |
| `arrows` | `{{at, text, side, kind}}` — `at` is a field property (→ that field), `@button:<label>`, or `@role:<sel>`; `text` rides the arrow; `side` is `left`/`right`; `kind` is `"arrow"` (default) or `"box"` (outline the target) |
| `clip` | bound the capture: omit for the **full page**; a **CSS selector** (`"#field-status"`, `".form-section"`) for that element; or the keyword **`"focus"`** for the bounding box of everything the `arrows` point at |
| `pad` | padding in px around a `clip` region (default `24`; `pad=0` for a tight crop). Clamped to the page, so the crop never extends past the content |
| `out`, `alt` | the image file (written next to the output) and its alt text |

**Cropping.** By default `screenshot{}` captures the whole page. To zoom in
on the subject of a figure, set `clip`:

```markdown
​```rela
-- one element, with breathing room:
screenshot{ ..., clip = "#field-status", pad = 32, out = "status.png" }

-- crop to exactly what the arrows highlight (fields + their labels):
screenshot{ ..., arrows = { {at="status", text="lifecycle state"} },
            clip = "focus", out = "focus.png" }
​```
```

`clip="focus"` unions the boxes of every annotated target **and** the drawn
arrow labels, so the crop includes the annotations, not just the fields.

**Prerequisites.** `screenshot{}` needs a **Chrome/Chromium browser** on
the machine and the **data-entry SPA built** into the `rela-docs` binary
(`just build-docs`, which runs `build-frontend` first). There is **no
graceful degradation**: if either is missing, the build fails loud — a
manual either captures a real figure or it errors, never a placeholder.

Because this browser dependency lives only in `rela-docs`, the everyday
`rela` and `rela-server` binaries stay lean and never link it.

**Fail-loud specifics.** An unknown field anchor, an entity that fails to
render for the chosen role (the form shows a load error), or a page taller
than the height cap each stop the build with the offending manual line.

Everything else in the build stays browser-free: a manual with **no**
`screenshot{}` never launches a browser or touches the data-entry app.
