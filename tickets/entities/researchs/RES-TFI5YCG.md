---
id: RES-TFI5YCG
type: research
title: Schema namespaces and variants as load-time sugar
summary: Namespace per included file with a reserved dot separator; variants are namespaced types extending an abstract root type; bare root names expand at load time and sibling conversions are generated; graph model unchanged
status: done
---

## Problem

A project often needs the same kind of entity in several domains. Example: a
todo in an ISMS and a todo in project management. The two are mostly the same.
They differ in their template, a few fields, their id prefix, and which
relations they may have.

Today there are two ways to model this, and both have a cost:

- **Separate types** (`isms-task`, `project-task`). Shared fields are copied
in the YAML, and every relation meant for "any task" must list every type.
- **One type with a hand-built discriminator.** The variant rules are spread
over the schema, forms, validations and templates. Relation endpoints cannot be
narrowed to one variant.

A third need comes with it: splitting the schema by domain, so that all ISMS
definitions live in `isms.yaml` and all project definitions in `projects.yaml`.

The question is how to give schemas this structure without changing rela's graph
model.

## Context

What rela does today:

- **Type names share one flat space.** Every file loaded through `includes:`
shares it. A duplicate name fails the load (`internal/metamodel/include.go`).
- **The root `namespace:` key holds an IRI, and nothing uses it.** Only
`rela schema` prints it (`internal/metamodel/types.go`,
`internal/cli/schema.go`). Included files may not set it (`include.go:148`).
- **The type-name check is a blocklist.** It rejects only quotes, backslashes
and control characters (`internal/metamodel/validation.go:32`). Dots and colons
are already allowed. No tracked schema uses either in a type name.
- **Folders and URLs are built from `plural`, not from the type name**
(`types.go:320`). Folders are `entities/<plural>/`, URLs
`/api/v1/<plural>/<id>`. When `plural` is not set, it is the name plus "s".
- **Template variants are per type**
(`templates/entities/<type>--<variant>.md`).
- **Relation endpoints and cardinality are per type.** `from:`, `to:`,
`min_outgoing` and `max_incoming` name types.
- **Conversions exist** (`conversions:`). A conversion changes an entity's
type with an explicit mapping of fields, relations and faces. It reuses the
rename machinery: aliases keep old links working, history keeps its lineage, and
ACL `convert:` grants apply. Unmapped properties are not carried forward.
Automations on the destination type do not run.
- **The loader uses yaml.v3**, which supports anchors and `<<:` merge keys
within one file. No schema uses them. They avoid copied property lists, but they
cannot expand relation endpoints and they do not work across included files.

## Options

### A. Separate types with a naming convention (status quo)

- Pros: no work. Relation rules and templates are exact and native.
- Cons: shared fields are copied. Every "any task" relation, ACL grant and
cross-domain config lists every type. The schema cannot be split by domain
without name collisions.
- Effort: none.

### B. One type with a discriminator, hand-built or generated

A `domain` enum, plus per-domain forms, validations and templates. A `variants:`
block could generate these.

- Pros: works today. Cross-domain lists come for free.
- Cons: relation rules per variant are enforced by validation, not by the
schema's endpoint check. One id prefix for all variants. The API schema shows
every variant's fields on every task. The variant is a property, while in
practice it is part of what the entity is.
- Effort: configuration only, or medium for a generator.

### C. Namespaces and variants as load-time sugar (recommended)

Each domain file declares a namespace. Its names are prefixed with a dot. A
namespaced type can extend an abstract root type, and that makes it a variant.
After loading, only ordinary concrete types exist, so the store, tracer, search,
ACL evaluation and every backend stay unchanged.

```yaml
# schema.yaml
iri: "https://example.org/ontology#"   # renamed from namespace:
includes: [isms.yaml, projects.yaml]

entities:
  task:
    abstract: true          # shared shape only; stores no entities
    plural: tasks
    properties:
      title: { type: string, required: true }
      status: { type: task_status }
      due: { type: date }

relations:
  assigned-to:
    from: [task]            # bare: expands to isms.task, projects.task
    to: [person]
```

```yaml
# isms.yaml
namespace: isms

entities:
  task:                     # becomes isms.task
    extends: task           # a type cannot extend itself: this is the root task
    id_prefix: ISMS-
    properties:
      evidence: { type: string }
  control:                  # becomes isms.control (namespaced, not a variant)
    id_prefix: CTL-

relations:
  addresses:                # becomes isms.addresses
    from: [task]            # resolves locally: isms.task only
    to: [control]           # isms.control
    min_outgoing: 1
```

```yaml
# projects.yaml
namespace: projects

entities:
  task:
    extends: task
    id_prefix: PRJ-
    properties:
      milestone: { type: string }
```

#### Rules

1. **The separator is a dot, and it is reserved.** A dot is safe in Windows
file names and unencoded in URLs, and it reads as a namespace. A dot in a
hand-written type name or `plural` is rejected. Alternatives considered:
   - `-` and `_` are already used inside names, so `a-b` would be ambiguous.
   - `~` is awkward to type and expanded by shells at the start of a word.
   - `@` is taken by faces.
   - `+` means a space in query strings.
   - `:` is not allowed in Windows file names. In a relative URL it reads as
a scheme, and `encodeURIComponent` encodes it, which gives one URL two
spellings.
2. **Every name in a namespaced file gets the prefix.** That covers entity
types, relation types, custom types and validations. Inverse names get it too,
which keeps them globally unique.
3. **Names resolve locally first, then in the root.** Inside `isms.yaml`,
`task` means `isms.task`, and `person` falls through to the root `person`. A
qualified name (`projects.task`) always works. So a relation declared in a
namespace file is narrowed to that namespace's types without extra syntax.
4. **A bare root name expands to its variants.** Wherever a config file names
an abstract type, the loader replaces it with every concrete type that extends
it: relation endpoints, validations, automations, conversions, `acl.yaml` and
`data-entry.yaml`. Expansion keeps cardinality's meaning, because cardinality is
counted per entity. The relation stays one relation type with one inverse name,
and only its endpoint lists grow.
5. **Extending merges the shape.** A variant inherits the parent's properties
and faces. It may add properties and override settings such as `required` and
`default`. It may not remove inherited properties.
6. **Generated values use only the dot:**
   - `plural` becomes `isms.tasks`, so the folder is `entities/isms.tasks/`
and the URL `/api/v1/isms.tasks/ISMS-1`.
   - The template is `templates/entities/isms.task.md`.
7. **Conversions between siblings are generated on opt-in.** Types that extend
the same root type get a conversion for each pair:
   - Inherited properties map to themselves. So do variant properties that
both sides declare with the same type.
   - A relation maps to itself when the target type is allowed at that end,
and is dropped otherwise.
   - Faces map by name.

A conversion declared by hand for the same pair takes precedence. Generated
conversions must pass the same load checks as hand-written ones.
8. **At runtime only concrete types exist.** `rela list task`, Lua, MCP and
REST reject an abstract name with an error that lists its variants. In config
predicates, `entity.type == 'task'` is rewritten to a membership test, so it
does not silently match nothing.
9. **Rename the root `namespace:` key to `iri:`,** with a project-file
migration, so `namespace:` can mean the name prefix in included files.

#### Pros

- Removes copy-paste and splits the schema by domain.
- Relation rules, id prefixes, templates, ACL grants and API schemas are
exact for each variant, because each variant is a real type.
- The graph model, storage and every backend stay unchanged.
- `rela schema` shows the expanded result, so the sugar can be explained and
checked.
- Runtime polymorphism can be added later without changing stored data.

#### Cons

- Changing variant is a conversion, not a property edit. The id changes with
the prefix, although aliases keep old links working.
- Errors must point back to the source ("`isms.task`, from `isms.yaml:6`,
extends `task`"), not only to the expanded result.
- An "all tasks" screen needs lists, views and kanbans that accept several
types. Today a list takes one `entity_type`. Expanding a single `entity_type:`
cannot produce one screen over several types, so this is a separate runtime
feature.

#### Effort

Medium:

- loader work: namespace prefixing, name resolution, `extends:` merging,
expansion, generated `plural` and conversions;
- expansion in the `acl.yaml` and `data-entry.yaml` loaders;
- the `iri:` migration;
- error messages that point back to the source.

### D. Runtime subtyping

Keep the parent type at runtime, so that `rela list task` and `entity.type`
queries match every subtype.

- Pros: cross-domain reads work on every surface.
- Cons: every subsystem that reads type names must understand the parent and
child relationship. That changes the graph model, which is what this research
set out to avoid.
- Effort: large.

## Recommendation

Build option C. Namespaces are declared per included file, the separator is a
reserved dot, a variant is a namespaced type that extends an abstract root type,
a bare root name expands to all variants in config, and conversions between
siblings are generated on opt-in. After loading, only ordinary types exist.

Open points to settle in design review:

- **Who names the namespace.** The file (`namespace: isms`) or the include
(`{file: isms.yaml, as: isms}`). The file is easier to read, and the include
would allow loading one pack twice. Start with the file.
- **Nesting.** Start with one level (`ns.name`).
- **Abstract root only.** Allowing `extends:` only on an abstract root keeps a
bare name meaning exactly "all variants". A concrete parent would mean both
itself and its variants.
- **Field loss in the conversion preview.** The preview should list the
properties a conversion will drop. A generated mapping makes silent loss likely,
for example `evidence` when converting an ISMS task to a project task.

Follow-up outside this design: lists, views and kanbans that accept several
entity types, for cross-domain screens such as "all my tasks".
