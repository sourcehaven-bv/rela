<!-- This file is auto-generated from docs-project/entities/. Do not edit directly. -->

# Importing structured data with a mapping

`rela import --mapping` reads a structured file that was not made for rela,
such as a control catalog in OSCAL, a CSV export from a spreadsheet, or an
XML feed. A mapping file says which parts of the file become which entities
and relations. You write the mapping once and can run it again whenever the
source changes.

Each run first makes a plan: the entities to create, the properties to
change, and the relations to add or remove. With `--dry-run` the plan is
all you get. Without it, rela writes the plan through the normal write path,
so validation, automations, the audit log and your `acl.yaml` apply to
every change. A second run of the same file plans nothing.

## A first mapping

Mapping files live in `imports/` in the project root. The file name is the
import name: `imports/catalog.yaml` is the import `catalog`.

Take this input, `controls.json`:

```json
{
  "controls": [
    { "id": "AC-1", "title": "Access policy", "family": "AC" },
    { "id": "AC-2", "title": "Account management", "family": "AC" }
  ]
}
```

And a schema with a `control` type whose `code` property is `unique: true`.
This mapping turns each item of `controls` into a control:

```yaml
description: Controls from the catalog export
entities:
  controls:
    type: control
    select: $.controls
    key: code
    properties:
      code: id
      title: title
```

Check what it would do, then run it:

```bash
rela import controls.json --mapping catalog --dry-run
rela import controls.json --mapping catalog
```

`rela validate` checks every file in `imports/` against the schema, so a
typo in a type or property name shows up before you run an import.

## How a record finds its entity

An entity mapping usually has a `key`: a property that identifies the entity
across runs. The key property must be `unique: true` or an `external_ref`,
and the mapping must fill it under `properties`. On each run, rela looks up
the stored entity with that key value:

- No entity has the key: the record is created.
- One entity has it: the record updates that entity. Only properties whose
  value changed are written. Properties the mapping does not name are left
  alone.
- The key is held by an entity outside the mapping's `scope`, by several
  entities, or by an entity you may not read: the record is an error and is
  left out.

For a type with `id_type: manual`, set `id:` to a template such as
`"ctl-{{id}}"`. The template gives the id of new entities. Without a `key`,
the id is also how a record finds its entity.

An entity whose key is no longer in the source is listed as missing in the
plan. The import never deletes an entity.

## Mapping reference

Top-level fields:

| Field         | Meaning                                                                   |
| ------------- | ------------------------------------------------------------------------- |
| `description` | Free text                                                                 |
| `format`      | `json`, `yaml`, `xml` or `csv`. Default: from `--format` or the extension |
| `max_bytes`   | Largest input accepted. Default 10 MB, at most 50 MB                      |
| `max_records` | Most records one entity mapping may select. Default 10000, at most 100000 |
| `entities`    | The entity mappings, by name                                              |

Fields of an entity mapping:

| Field        | Meaning                                                                       |
| ------------ | ----------------------------------------------------------------------------- |
| `type`       | The entity type                                                               |
| `select`     | A path from the document root. Each node it selects is one record             |
| `key`        | The identifying property: `unique: true` or `external_ref`                    |
| `id`         | A template for the id of new entities (types with `id_type: manual` only)     |
| `scope`      | Fixed property values. Only entities with these values are matched or missing |
| `properties` | Property name to value                                                        |
| `content`    | A value for the markdown body. When set, the import owns the body             |
| `relations`  | Relations to derive from each record                                          |
| `hook`       | A Lua script that adjusts each record                                         |

### Values

A value is one of three things:

```yaml
properties:
  title: title # a path, relative to the record
  label: { template: "{{family}}-{{id}}" } # a template with paths in {{ }}
  source: { value: catalog } # a constant
```

A path that matches several values is an error for a property that is not
a list. Set `join` to combine them, as in `{path: notes, join: "\n"}`. For
`content`, several matches are joined with a blank line.

When the source has no value for a property, the stored value stays. Set
`clear_missing: true` on the value to remove it instead. It is off by
default because a filtered export would otherwise empty the property on
every entity.

Values are converted to the property type: integers (`3`, `3.0` and `3e0`
are all 3), booleans and external refs (the source value becomes the ref's
id). Scope values and values the hook sets are converted the same way. Other
types are passed as text and checked by the schema's validation.

### Paths

Paths are deliberately small. They have no functions and no arithmetic.

| Syntax      | Meaning                                                        |
| ----------- | -------------------------------------------------------------- |
| `$`         | The document root, as the first step                           |
| `.`         | The record itself                                              |
| `name`      | A field. For XML: a child element, or `@name` for an attribute |
| `['a b']`   | A field whose name has spaces, dots or other characters        |
| `*`         | Every field value or array item                                |
| `..name`    | The field at any depth below                                   |
| `^`         | The nearest enclosing object, skipping arrays                  |
| `[f=value]` | Keep nodes whose field `f` has this text; quotes are optional  |
| `[f]`       | Keep nodes that have the field `f`                             |

Steps are joined with dots, for example `props[name=label].value` or
`^.id`. A step that reaches an array yields its items, and a path yields
each node once. A quoted filter value may hold `]`. For a CSV column such
as `Control ID`, write `['Control ID']`.

How each format becomes a tree:

- JSON and YAML keep their own structure. YAML anchors are expanded; merge
  keys (`<<`) are not.
- XML: the root is an object holding the root element. Attributes are
  `@name` fields. Child elements become lists by name. An element's text,
  with whitespace collapsed, is its value when a path ends on it, and
  `#text` reads it explicitly. An empty element such as `<title/>` has no
  value.
- CSV: the first row is the header. The root is a list of row objects, so
  `select: $` selects every row.

### Relations

```yaml
relations:
  - type: part-of
    target: groups # another entity mapping in this file
    key: ^.id
  - type: mitigates
    target_type: risk # a type this file does not import
    target_key: ref # its unique or external_ref property
    key: risks
    optional: true
```

`key` gives the target's key. Several matches make several relations. With
`target`, the key is looked up among that mapping's records first, so a
relation can point to an entity created in the same run. With `target_type`
and `target_key`, the target must already exist.

| Field       | Meaning                                                                |
| ----------- | ---------------------------------------------------------------------- |
| `name`      | Name of the relation in the hook's draft. Default: the type            |
| `direction` | `out` (record to target, the default) or `in`                          |
| `optional`  | A record without a target key, or with an unknown one, is not an error |
| `prune`     | Remove stored relations of this type to targets the source drops       |

Without `prune`, a stored relation the source no longer names stays and is
listed as stale in the plan. When two relation mappings of one entity
mapping share a relation type and direction, neither may set `prune`: the
pruning one would remove what the other adds.

A target key is converted like the target's key property, so `007` finds an
integer key 7.

### Scope

Two imports can share a type. Give each a `scope` such as
`{catalog: iso27002}`. New entities get the scope values. Matching, missing
reports and pruning only look at entities with those values. Two entity
mappings of the same type in one file need scopes that set one property to
different values.

### The Lua hook

A hook adjusts records the mapping language cannot express. It runs once per
record:

```yaml
hook: scripts/catalog.lua # function transform(draft, record)
# or: hook: {script: scripts/catalog.lua, function: shape}
```

```lua
function transform(draft, record)
  if record.status == "withdrawn" then
    return nil -- skip this record
  end
  draft.properties.title = string.upper(draft.properties.title)
  return draft
end
```

`draft` has `type`, `key`, `id`, `properties`, `content` and `relations`
(relation name to a list of target keys). `record` is the source record as
plain tables. Return the draft to keep the record, or `nil` to skip it. The
hook may not change the key.

The hook runs in a sandbox with the safe standard libraries and `require`
for modules in `lib/`. It has no rela bindings, no file access and no
network, so it cannot read the graph or write. `print` writes to standard
error. One Lua state serves every record of the mapping, so a global set
for one record is still there for the next.

## Running an import

```bash
rela import <file> --mapping <name> [--dry-run] [--skip-errors] [-o json]
```

- `--dry-run` shows the plan and writes nothing.
- `-o json` prints the plan, and after a run the result, as JSON.
- A record with an error is reported with its place in the file, such as
  `$.catalog.groups[0].controls[2]`, or the line for CSV. A record with an
  error is left out whole, with its relations, and so is a record whose
  required relation points to it. When the plan has errors, nothing is
  written. `--skip-errors` writes the rest.
- A write the entity manager refuses, for example because your role may not
  create that type, is reported and the rest goes on.

## Safety

The mapping is operator-written configuration. The input file is not
trusted:

- The input may be at most `max_bytes`, and the decoded tree is capped in
  depth and in node count. YAML aliases count against the cap, and a YAML
  file with more lines and separators than the cap is refused before it is
  parsed.
- XML is read without a DTD, so external entities and entity expansion
  ("billion laughs") are refused.
- The import runs no external commands and the hook has no network.
- The plan names a stored entity only when you may read it, and shows only
  property values you may see. A record that would compare or write a
  property hidden from you by a `visible:` rule is an error. An entity that
  hides its key or scope property from you counts as one you cannot read.

## Example: an OSCAL catalog

OSCAL catalogs, such as ISO 27002 or BIO2 in OSCAL form, hold groups of
controls. Controls can nest: an enhancement sits inside its parent control.
This mapping imports groups and controls, links each control to its group,
and links an enhancement to the control it refines:

```yaml
format: json
entities:
  groups:
    type: control-group
    select: $.catalog.groups
    key: oscal_id
    scope: { catalog: iso27002 }
    properties:
      oscal_id: id
      title: title
  controls:
    type: control
    select: $.catalog.groups..controls
    key: oscal_id
    scope: { catalog: iso27002 }
    properties:
      oscal_id: id
      title: title
      label: props[name=label].value
    content: parts[name=statement].prose
    relations:
      - type: part-of
        target: groups
        key: ^[class=category].id
        optional: true
      - type: refines
        target: controls
        key: ^[class=control].id
        optional: true
    hook: scripts/oscal.lua
```

For the XML form of the same catalog, the paths use attributes and element
names: `select: $.catalog..control`, `oscal_id: "@id"`,
`label: prop[@name=label].@value` and `content: part[@name=statement].p`.

## Limits

- Types with `faces:` cannot be imported yet.
- Relation properties are not mapped.
- Every created or updated entity runs its automations, as any write does.
  A large first import of a type with many automations takes time.
