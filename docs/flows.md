<!-- This file is auto-generated from docs-project/entities/. Do not edit directly. -->

# Flows (flows.yaml)

`flows.yaml` declares the flows of a project. A flow is a process that
follows one entity, its subject, through a series of states. An
onboarding flow, for example, follows a person from "collect documents"
to "finished", and creates a task on the way.

The file is optional. Without it, a project has no flows and nothing else
changes. `schema.yaml` never refers to it.

> **Status.** This release loads and checks `flows.yaml` and shows its
> flows. rela does not run flows yet; starting runs, the run records and
> the inbox entries follow in later releases. You can write and check the
> file now.

## Example

```yaml
flows:
  onboarding:
    label: Onboarding
    subject: person
    run_as: system:onboarding-bot
    per_subject: one
    deadline: 30d
    retention: { after: 90d, action: delete }
    start:
      - on: create
        when: "entity.status == 'hired'"
      - on: manual
        permission: onboarding.start
    initial: collect
    states:
      collect:
        label: Collect documents
        waiting_for: [hr]
        outputs:
          - set: { properties: { stage: collecting } }
          - create:
              type: task
              properties:
                title: "Collect documents for {{subject.name}}"
                due: "{{today}}"
              relate: { type: about }
        on:
          change:
            to: review
            roles: [hr]
            when: "entity.documents == true"
        after:
          stalled: { period: 7d, to: escalated, label: No documents in a week }
      review:
        on:
          done: { to: finished }
      escalated:
        status: input_required
        waiting_for: [manager, hr]
        on:
          change: { to: review, when: "entity.documents == true" }
      finished:
        final: true
```

## Flow keys

| Key | Meaning |
| --- | ------- |
| `label` | The name people see. Defaults to the flow's name. |
| `subject` | The entity type a run follows. Required. Name the type itself, not an alias. A type with faces is not supported yet. |
| `run_as` | The job identity the flow's outputs write as. Required when a state has outputs. acl.yaml must grant it every write the outputs make. |
| `per_subject` | `one` (the default) allows one open run per subject; `many` allows several. |
| `deadline` | How long a run may take, such as `30d`. At least one minute. |
| `retention` | When to clean up a finished run: `after` (a period, required) and `action` (`archive` or `delete`). |
| `start` | The triggers that start a run. See below. |
| `initial` | The state a run starts in. Required. It may not be final. |
| `states` | The states, by name. Required. |

A flow, state or timer name is 1 to 64 lower-case letters, digits, `-` or
`_`, and starts with a letter. Periods use the same format as
`realtime.yaml` timers: a number and a unit (`s`, `m`, `h`, `d`, `mo` or `y`), such as `90m`, `7d` or `3mo`.

## Start triggers

Each entry in `start:` names one way to start a run.

| `on:` | Starts a run | Other keys |
| ----- | ------------ | ---------- |
| `create` | when a subject is created | `when` (optional) |
| `predicate` | when a subject comes to match `when` | `when` (required) |
| `manual` | when a person asks for it | `permission` (required) |
| `schedule` | on each subject that matches `when`, every period | `every` (required), `when` (optional) |

A manual start needs a permission from acl.yaml. rela warns when no role
grants it, because then nobody can start the flow by hand. rela also warns
about a flow without start triggers. A schedule start needs
`per_subject: one`; with `many`, every tick would start another run on
every matching subject.

## States

| Key | Meaning |
| --- | ------- |
| `label` | The name people see. Defaults to the state's name. |
| `final` | `true` ends the run in this state. A final state has no `on:`, `after:` or `waiting_for:`. |
| `status` | `working` or `input_required` for a state that is not final; `completed`, `cancelled` or `failed` for a final one. The defaults are `working`, `input_required` when `waiting_for:` is set, and `completed`. |
| `waiting_for` | The acl.yaml roles the run waits on in this state. Each role must be able to move the run on through an `on: change` branch. |
| `outputs` | What rela writes when a run enters the state. See below. |
| `on` | The events that move the run on. See below. |
| `after` | Timers, by name, that move the run on. See below. |

Every state is reachable from `initial`, and every state that is not final
has a way out. rela refuses a file where either is not so.

### Events

`on:` maps an event to one branch or a list of branches. rela tries the
branches in order and takes the first that applies.

- `change`: the subject changed. A branch can name `roles`, the acl.yaml
  roles one of which the person who made the change must hold, and a
  `when` over the changed subject.
- `done`: rela finished writing the state's outputs. A state without
  outputs is done as soon as a run enters it. A `done` branch takes no
  `roles`, because nobody causes it.

A branch has `to` (the next state, required), and optional `label`,
`roles` and `when`. rela warns about a branch that can never be taken
because the branch before it has no `roles` and no `when`.

### Timers

```yaml
after:
  stalled: { period: 7d, to: escalated, label: No documents in a week }
  deadline: { period: 30d, since: run, to: failed }
```

A timer moves the run to `to` once `period` has passed. `since: state`
(the default) counts from when the run entered the state; `since: run`
counts from when the run started. A state has at most 8 timers.

### Outputs

An output writes when a run enters the state, as the `run_as` identity.

- `set` sets properties on the subject. A state has at most one `set`.
- `create` creates an entity of `type` with `properties`, and links it to
  the subject with the relation in `relate.type`. The relation must lead
  from the new type to the subject type. Name the type itself, not an
  alias. The type must generate its ids (not `id_type: manual`).

A property value is a literal, or a template with placeholders in
`{{...}}`:

| Placeholder | Value |
| ----------- | ----- |
| `{{subject.id}}` | The subject's id. |
| `{{subject.<property>}}` | A property of the subject. |
| `{{run.id}}` | The run's id. |
| `{{now}}` | The current time. |
| `{{today}}` | The current date. |

A value that is only one placeholder keeps the placeholder's type, so
`due: "{{today}}"` writes a date. An id, or text around a placeholder,
makes the value text, which only a text property accepts. A literal value
must be valid for its property, and a `create` must set every required
property that has no default.

A template may not read a subject property that a `visible:` grant in
acl.yaml restricts, or a roll-up, whose value depends on who reads it. An
output may not write a computed or roll-up property.

A `create` output copies what its templates read into a new entity. Who
can read that entity follows acl.yaml for its own type, not for the
subject. Like an automation that creates an entity, this is a choice the
author of `flows.yaml` makes: do not copy subject values into a type that
more people can read than the subject.

## Conditions

A `when:` is a condition over the subject, written in the same language as
`realtime.yaml` triggers. To keep a run's state from revealing values a
reader cannot see, a flow condition is held to a smaller set:

- It reads the subject only as `entity.<property>`.
- It does not follow relations (`related`).
- It calls only `contains`, `date_add`, `days_between`, `len` and `today`.
- It does not read a property that a `visible:` grant in acl.yaml
  restricts.

## Versions

Each flow has a version: a hash of its definition. A change to a flow
changes its version; a change to a comment does not. `rela flow show`
prints it. Runs will keep the version they started with.

## Checking the file

```bash
rela flow validate
rela flow show
rela flow show onboarding
```

`rela flow validate` loads `flows.yaml` against the schema and acl.yaml
and prints each problem with its key, such as
`flows.onboarding.states.review.on.done.to: names "finish", which states
does not declare`. `rela validate` runs the same check. `rela flow show`
lists the flows, and with a name prints that flow as a Mermaid state
diagram.

Names starting with `_` are reserved in `schema.yaml` for the types rela
adds for flows. See [Metamodel](metamodel.md).
