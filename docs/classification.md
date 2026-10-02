<!-- This file is auto-generated from docs-project/entities/. Do not edit directly. -->

# Data Classification

`classification.yaml` records what kind of data each field in your schema
holds: a name, a contact detail, a health note, or nothing sensitive at all.
rela uses the file only to answer questions and to warn. It changes no
behavior, and it is not access control.

The file is optional. A project without it works exactly as before.

## Why a separate file

Classification is a concern of some projects, not all. Keeping it out of
`schema.yaml` keeps the schema small, and a project that does not need it
never sees it. The CLI creates and updates the file, so it does not drift
from the schema unnoticed.

Three layers stay apart:

| Layer        | Where                                    | Says                             |
| ------------ | ---------------------------------------- | -------------------------------- |
| Describe     | `classification.yaml`                    | what data a field holds          |
| Behave       | `schema.yaml`, `acl.yaml`, other config  | what rela does with a field      |
| Check        | `rela classification`, `rela validate`   | whether the two fit together     |

Nothing in `classification.yaml` makes rela hide, redact, or refuse
anything. If a field holds personal data and you decide to show it, that is
your decision. The tools report it; they do not block it.

## The file

```yaml
labels:
  name:       { role: direct-identifier }
  contact:    { role: direct-identifier, description: Email or phone. }
  birth-date: { role: quasi-identifier }
  postcode:   { role: quasi-identifier }
  gender:     { role: quasi-identifier }
  salary:     { role: attribute, meta: { owner: hr } }
  health:     { role: attribute, reference: "GDPR art. 9" }

  identified-person:
    role: direct-identifier
    when:
      any_of:
        - all_of: ["@direct-identifier", "@quasi-identifier"]
        - count: { of: "@quasi-identifier", min: 3 }

assign:
  person:
    title: [name]
    email: [contact]
    body: none
  employment:
    birth_date: [birth-date]
    postcode: [postcode]
    gender: [gender]
    salary: [salary]
    body: needs-review

assign_relations:
  reports-to:
    note: none
```

### Labels

A label is a name you choose for a kind of data. rela attaches no meaning to
the name. Use the terms your organization already uses; `health` and
`special-category` are equally valid, but pick one rather than both.

A label name is 1 to 64 characters: lowercase letters, digits, and hyphens,
starting with a letter or digit. The names `none`, `needs-review`,
`direct-identifier`, `quasi-identifier` and `attribute` are reserved.

Each label may have:

- **`role`**: how the data relates to a person.
  - `direct-identifier`: identifies a person on its own (a name, an email).
  - `quasi-identifier`: identifies a person only in combination (a birth
    date, a postcode).
  - `attribute`: says something about a person without identifying them (a
    salary, a diagnosis).
- **`description`** and **`reference`**: free text for readers, such as the
  legal article a label stands for.
- **`meta`**: any mapping, kept for your own tooling. rela ignores it.
- **`when`**: a combination rule. See below.

### Combination rules

Some data is sensitive only in combination. A birth date on its own says
little; a birth date next to a name identifies a person. A label with a
`when:` rule is a **derived label**: rela applies it wherever the rule
holds, and you do not assign it to fields yourself.

A rule has one of three conditions:

- `all_of: [a, b, ...]`: every item holds.
- `any_of: [a, b, ...]`: at least one item holds.
- `count: { of: a, min: N }`: at least N fields match `a` (N is 1 to 64).

An item is a label name, a role written as `"@direct-identifier"` (quote
it, because `@` cannot start a plain YAML value), or a nested condition.
Rules nest up to 8 levels.

A rule may name another derived label, including through its role, so
derived labels can build on each other.

`scope` says where the fields must occur together:

- `record` (the default): in one entity, or one relation.
- `subject`: anywhere in the data about one person. This uses the subject
  inference described below.

### Assignments

`assign` lists every entity type and, under it, every field. A field is a
declared property or `body`, the markdown content. `assign_relations` does
the same for relation types, including `body` when the relation type has
`content: true`.

Each field takes one of three values:

- A list of labels, such as `[name]` or `[contact, name]`. Always a list,
  even for one label.
- `none`: reviewed, and holds nothing you classify.
- `needs-review`: not reviewed yet. `rela classification sync` writes this
  for every new field. Lint and `rela validate` fail while any remain.

An empty list is an error; write `none` instead. The explicit states exist
so that "not sensitive" and "not looked at" can never be confused.

### Subjects

A **subject** is an entity type that represents a person, such as `person`
or `employee`. rela infers subjects: an entity type is a subject when one of
its own fields has a label with the `direct-identifier` role, directly or
through a derived label with `scope: record`.

Data is about a subject when it sits on the subject itself or on an entity
linked to it. A **subject link** is any relation type that connects to a
subject type. rela follows subject links 1 hop by default and stops at any
other subject, so a relation between two people never merges them into
one.

Three keys override the inference:

```yaml
subject:
  company: false        # has a contact name, but is not a person
subject_link:
  watches: false        # following a topic is not data about the topic
  in-department: true   # employment --in-department--> department
subject_hops:
  in-department: 2      # followed up to 2 hops from the person
```

`subject_hops` sets how far from the subject a relation may be followed:
1 (the default) means only from the subject itself, 2 also from a type one
hop away, up to 3. In the example, the profile of `person` reaches
`employment` through `employment-of`, then `department` through
`in-department`. A relation that does not touch a subject type needs
`subject_link: true` as well; lint reports a `subject_hops` entry that has
no effect (`no-effect`).

`rela classification report` shows each subject, the reason rela treats
it as one, and the types its profile reaches.

A profile holds at most 64 entity types. A larger one is cut off and
marked as truncated.

### File rules

- The file is at most 1 MiB and declares at most 256 labels.
- YAML anchors, aliases and merge keys (`<<`) are refused. `sync` edits the
  file, and an edit through an alias would change several places at once.
- Keys must be plain strings, and a key may appear only once per mapping.

## Commands

### rela classification sync

Creates `classification.yaml` if it does not exist, and otherwise brings it
in line with the schema:

- Adds every field without an entry as `needs-review`, next to its
  neighbors in schema order.
- Follows renames recorded in `migrations/` (`rename_property`,
  `rename_entity_type`, `rename_relation_type`) and moves the entry to the
  new name. It moves an entry only when the old name is gone from the
  schema, so a new type that reuses an old name keeps its own entry.
- Reports entries the schema no longer has as **stale**. Sync never deletes
  anything; remove stale entries by hand.
- Reports a **conflict** when a rename's new name already has an entry, and
  leaves both alone.

Sync keeps comments and key order. It does not keep blank lines or custom
indentation. It refuses to edit a file that does not parse cleanly.

```bash
rela classification sync            # write the file
rela classification sync --dry-run  # report only
```

Relation property renames have no migration step, so sync reports the old
name as stale and adds the new name as `needs-review`.

### rela classification lint

Checks the file against the schema and exits 1 on any issue:

- a type or field without an entry;
- a field still marked `needs-review`;
- a stale type or field;
- a `subject`, `subject_link` or `subject_hops` key the schema does not
  have;
- any parse error: syntax, unknown keys, undefined labels, invalid rules.

Each issue names the file line. With `-o json`, the issues are in
`details`, each with `code`, `path`, `line` and `message`.

A project without the file passes, with a note.

### rela classification report

Shows what the classification says about the schema:

- **Subjects**: each subject type, why it is one (the field and label, or
  the override), the types it reaches and by which relations, and the
  subject-scope derived labels its profile carries.
- **Entity types** and **relation types**: every field with its labels or
  state (`none`, `needs-review`, or `missing`), and the record-scope derived
  labels. A relation type is marked when it is a subject link.
- A subject type whose ids are not opaque (`id_type: manual`) is flagged,
  because the id itself may identify a person.

Each derived label names the fields that made its rule hold. A record-scope
label used inside a subject rule is expanded to its own fields.

```text
Subjects
  person: person.title has name (direct-identifier)
    reaches sick-leave via <- of sick-leave
    derived identified-health from person.email, person.title, sick-leave.diagnosis
```

The report works on a file with lint issues, so you can use it while
reviewing. It refuses a file that does not parse. With `-o json`, the
report is in `details` with `types`, `relations` and `subjects`, each
sorted by name.

### rela acl audit

`rela acl audit` adds low-severity findings about what each role can read
when `classification.yaml` exists. They never change the exit code, not
even with `--fail-on=any`: they list access you may well intend, and there
is no way to suppress one. `--no-classification` leaves them out. A file
that does not parse is skipped with a warning.

The audit evaluates each role alone, including `everyone`, and each role
through every client baseline: with no scopes, with each scope, and with
all scopes. It evaluates them with the same code a request uses, and counts
every conditional grant (`when:`) as granted, so each finding is the worst
case for that view.

Two rules keep the list short without making silence ambiguous:

- What `everyone` reads is listed once, under `role everyone`, and not
  again under each role, since every principal holds `everyone`.
- A client view that reads exactly what its role reads gets one
  `C0-same-as-role` finding. One that reads no labeled data gets
  `C0-reads-none`. Any other client view lists all its findings, so a
  finding missing there is one the client ceiling blocks.

| Rule | Finding |
| --- | --- |
| `C1-role-reads-label` | The view reads fields with this label. |
| `C2-derived-record` | The view can combine fields in one record into a derived label. |
| `C3-derived-subject` | The view can combine fields about one subject into a derived label. |
| `C0-same-as-role`, `C0-reads-none` | A client view matches its role, or reads nothing labeled. |
| `C0-truncated` | The view has more than 200 findings. Combinations are kept first. |

A combination finding names the fields that make the rule hold. Its fix
lists the fields that break the combination when hidden alone. When no
single field does, it gives the fewest fields that must be hidden together,
up to 3. With `-o json`, each finding also carries `label` and `fields`.

Wrapped here to fit:

```text
  [low] role hr can combine person.name, leave.why in data about one person into
  health (C3-derived-subject: subject person)
      fix: hiding any one of person.name, leave.why breaks the combination
```

A finding does not say the policy is wrong. Whether a role should read a
label is your decision; the finding makes sure you decide it knowingly.

### rela validate

`rela validate` runs the same lint when `classification.yaml` exists.

## Limitations

- The commands read `schema.yaml`, `classification.yaml` and `migrations/`
  from disk and open no store. They cannot see configuration that lives
  only in a database.
- The audit checks what a view can read, not what it can write or which
  individual entities a conditional grant admits.
- The audit evaluates each role on its own (plus `everyone`). A user can
  hold several roles: through `asserted_role_assignments`, or through a
  role conferred on one record by `role_relations`. Such a user may combine
  fields that no single role reads; the audit does not report that
  combination.
- Entity ids are not fields. A manual id such as `jane-doe` can itself
  identify a person. The report flags such subject types; no entry in the
  file can label an id.
