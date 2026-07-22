<!-- This file is auto-generated from docs-project/entities/. Do not edit directly. -->

# Generated documentation: the rela docs language

`rela docs build` renders a **manual** — ordinary Markdown you write by
hand — resolving embedded **Lua islands** that pull mechanical reference
fragments from the deployment's schema (and a small in-memory graph the
manual seeds). Prose stays prose; the field tables, enum meanings, state
diagrams, relation graphs, and role matrices are generated so they can
never drift from `metamodel.yaml` / `acl.yaml`.

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
| `lifecycle{type, field}` | a mermaid `stateDiagram-v2` when the field's custom type declares `transitions:`, else a flat value list |
| `entity{id, fields}` | one **seeded** entity's fields |
| `count{type}` | the number of seeded entities of a type (echo-friendly) |
| `roles_matrix{type}` | a role × verb capability table from `acl.yaml` (omit `type` for every type) |
| `description()` | the metamodel's top-level `description:` (echo-friendly) |
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

## Building

```bash
rela docs build manual.md            # resolved Markdown to stdout
rela docs build manual.md --out site/manual.md
rela docs build manual.md --strict   # fail if any island resolves to nothing
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
| `view` | `"form"` (edit form, default), `"entity"` (detail), `"list"` |
| `type`, `entity` | the entity type and the id of a **seeded** entity to render |
| `form` | the `data-entry.yaml` form id (default `edit_<type>`) |
| `as` | the ACL **role** to render as (a principal assigned that role); default picks a role that can edit |
| `arrows` | `{{at, text, side}}` — `at` is a field property (→ that field), `@button:<label>`, or `@role:<sel>`; `text` rides the arrow; `side` is `left`/`right` |
| `clip` | a CSS selector to capture just one element |
| `out`, `alt` | the image file (written next to the output) and its alt text |

**Prerequisites.** `screenshot{}` needs a **Chrome/Chromium browser** on
the machine and the **data-entry SPA built** into the binary (`just
build-frontend`). There is **no graceful degradation**: if either is
missing, the build fails loud — a manual either captures a real figure or
it errors, never a placeholder.

**Fail-loud specifics.** An unknown field anchor, an entity that fails to
render for the chosen role (the form shows a load error), or a page taller
than the height cap each stop the build with the offending manual line.

Everything else in the build stays browser-free: a manual with **no**
`screenshot{}` never launches a browser or touches the data-entry app.
