---
id: GUIDE-flows
type: guide
title: Flows (flows.yaml)
status: published
order: 29
audience: advanced
summary: "Declare multi-step processes over an entity in flows.yaml: states, events, timers and outputs, and how rela checks the file"
---

`flows.yaml` declares the flows of a project. A flow is a process that
follows one entity, its subject, through a series of states. An
onboarding flow, for example, follows a person from "collect documents"
to "finished", and creates a task on the way.

The file is optional. Without it, a project has no flows and nothing else
changes. `schema.yaml` never refers to it.

> **Status.** This release runs flows that a person starts by hand, from
> the web app, over MCP or with `rela flow start`. A run writes its
> outputs, follows `done` branches and ends in a final state. People do
> their steps in the web app or over MCP, and a completed step takes the
> `change` branches. The `create`, `predicate` and `schedule` triggers,
> `change` events from ordinary edits, timers, retention and the inbox are
> checked but not acted on yet; they follow in a later release. See
> [Running flows](#running-flows) and [Doing a step](#doing-a-step).

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
| `outside_edits` | Which subject properties people may change outside the flow while a run is open. See [Edits outside a flow](#edits-outside-a-flow). |

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
| `writes` | The subject properties the step form of this state edits, at most 32. Only a state with `waiting_for:` takes it. Each must be a property the subject type declares, and not a computed or rollup property. |
| `outputs` | What rela writes when a run enters the state. See below. |
| `on` | The events that move the run on. See below. |
| `after` | Timers, by name, that move the run on. See below. |
| `join` | Makes the state the join of parallel branches: `all`, `any` or `{ when: ... }`. See [Parallel branches](#parallel-branches). |

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

### Parallel branches

A branch whose `to:` is a list forks the run: the run goes into every
state of the list at once, and each goes on as a branch of its own. The
branches meet again at a state with `join:`.

```yaml
start:
  on:
    done: { to: [it, legal] }
it:
  waiting_for: [it_desk]
  writes: [laptop]
  on:
    change: { to: joined, roles: [it_desk] }
legal:
  waiting_for: [hr]
  on:
    change: { to: joined, roles: [hr] }
joined:
  join: all
  on:
    done: { to: finished }
```

A fork names at least two and at most 8 states. A branch is every state
its first state reaches before the join. `join:` says when the run goes
on past the join:

| Value | The run goes on when |
| ----- | -------------------- |
| `all` | every branch has reached the join. |
| `any` | one branch has reached the join. The other branches stop. |
| `{ when: ... }` | the condition holds, or every branch has arrived. Besides `entity`, it reads `arrived` (the branches that reached the join) and `branches` (the branches of the fork). |

Each branch waits, writes its outputs, takes its timers and takes
`change` branches like a run without a fork. While it is forked, the run
itself is in the join state, and its status is `input_required` when a
branch waits for a person. A branch that reaches a final state ends the
whole run in that state.

rela refuses a fork it cannot follow:

- every branch reaches exactly one join, the same one for the whole fork,
  and a join closes one fork;
- branches share no state, and no state outside a branch moves into one;
- a branch does not lead back to the state that forked, to the initial
  state, or to another fork: forks do not nest;
- from every state in a branch, the run can reach the join or a final
  state.

## Conditions

A `when:` is a condition over the subject, written in the same language as
`realtime.yaml` triggers. To keep a run's state from revealing values a
reader cannot see, a flow condition is held to a smaller set:

- It reads the subject only as `entity.<property>`.
- It does not follow relations (`related`).
- It calls only `contains`, `date_add`, `days_between`, `len` and `today`.
- It does not read a property that a `visible:` grant in acl.yaml
  restricts. acl.yaml must key `visible:` by the type's own name; a key
  that names an alias fails to load, because it would restrict nothing.

## Versions

Each flow has a version: a hash of its definition. A change to a flow
changes its version; a change to a comment does not. `rela flow show`
prints it. A run keeps the version it started with: rela stores each
version's definition, so a run started before a change finishes on the
old definition.

## Runs

A run is recorded as an entity of the type `_flow-run`, with id prefix
`RUN-`. rela adds the type when a project declares flows; `schema.yaml`
cannot declare it. A run holds the flow's name, its version, its status
(`working`, `input_required`, `completed`, `cancelled` or `failed`), its
active state and its start, completion and deadline times. Two relations
link it to the graph: `_carries` points at the subject, and `_produced`
points at each entity a step created.

**Who can read a run.** A run is readable exactly when its subject is.
rela follows the run's `_carries` edge and applies the subject's read
rules, including role relations and client ceilings. A run whose subject
you cannot read answers like a missing entity. A run with no subject,
or whose subject was deleted, is hidden from everyone but the engine. A
subject you read only through `inherit_roles_through` or a face-split
role relation does not show its runs yet. `rela acl who-can read` on a
run lists the routes by which each reader reads the subject.

**Who can write a run.** Only the flow engine, as `system:flow`. acl.yaml
may not name `_flow-run`, `_carries`, `_produced`, a role starting with
`_`, or `system:flow`, and `"*"` in a grant does not cover them. The rule
also holds for `rela.bypass_acl(...)`, for a project without acl.yaml,
and for `--run-as`, which refuses `system:flow`. Nobody renames or
converts a run. Automations and validation rules do not fire on runs;
one that names a run type or relation fails to load. `rela import` and
data migrations refuse them too. `comments: on: ["*"]` does not make
runs commentable. Deleting a subject removes its runs' `_carries`
edges; the runs stay. Converting a subject keeps the edge when the
target type can be a subject, and drops it otherwise. A hand edit of the markdown files on
the filesystem backend is outside these rules.

**Audit.** Every write a flow step makes carries `flow`, `run`, `step`
and `source: flow` in the audit log. A write made by a start, a cancel, a
retry or a completed step also carries `by`: the user who asked. The
values a completed step writes are recorded as that user's own write. See
[Audit log](audit-log.md).

## Running flows

```bash
rela flow start onboarding PER-12   # start a run on PER-12
rela flow runs                      # list the runs you can read
rela flow runs --flow onboarding
rela flow cancel RUN-4OSX           # cancel an unfinished run
```

`rela flow start` needs an `on: manual` trigger on the flow, and the
caller must hold its `permission` for the subject. A role held through a
role relation to the subject counts. A subject the caller cannot read
answers like a missing entity. `rela flow cancel` needs the same
permission. In a project without acl.yaml, everyone may start and cancel.

When a run starts, rela records the `_flow-run` entity and its `_carries`
edge as `system:flow`, and then enters the initial state:

1. It writes the state's outputs as the `run_as` identity. A template
   reads the subject as `run_as` reads it: if `run_as` cannot read the
   subject, every `{{subject.*}}` placeholder is empty.
2. It takes the first `done` branch that applies, and enters that state.
3. In a final state the run ends: the `_flow-run` entity records the
   status and completion time, and the run's working state is dropped.

A failed step leaves the run in its state with the rest of the step
pending: outputs not yet written, a `done` branch not yet taken, or the
end of the run not yet recorded. rela tries again later and never writes
an output twice. It waits 30 seconds after the first failure and doubles
the wait after each further failure, up to one hour. `rela flow start`
names the run when its first step failed, because the run exists.

If rela stops between recording a run and storing its working state, the
run is listed but cannot advance. `rela flow cancel` closes such a run
once it is a minute old.

With `per_subject: one`, a second start on the same subject fails while
the first run is unfinished. Deleting a subject does not cancel its run
yet.

**Where runs are kept.** The working state of an unfinished run is kept
in `.rela/` on the filesystem backend, in `rela.db` on SQLite and in the
database on PostgreSQL. `rela db import-fs` copies the unfinished runs
into the new database.

**Upgrading.** A `flows.yaml` that does not load now stops rela from
starting, as an invalid `realtime.yaml` does. Run `rela flow validate`
before you upgrade.

## Doing a step

A run in a state with `waiting_for:` waits for a person. The step is
done by someone who holds one of those roles on the subject, globally or
through a role relation.

**In the web app.** The entity page of a subject shows a Flows panel with
its runs. For each run it shows the state, why the run waits, and the
step form:

- which roles the state waits for, and whether you hold one;
- the timers of the state and when each fires;
- each `change` branch, its roles and whether its `when` holds now;
- a step whose outputs failed, with the number of failures and the next
  attempt;
- the flow as a diagram with the current state marked, and the timeline:
  the start and the latest moves, with who made each.

The step form edits only the properties in the state's `writes:`, and
sends only the ones you changed. When you complete the step, rela writes
those values on the subject as you, with your own update rights and field
rules, and then takes the first `change` branch whose `roles` you hold and
whose `when` holds. When no branch applies, the values are saved and the
run stays where it is.

Once the values are saved, the step is not refused any more. If the run
then does not follow cleanly, the answer says why in `problem`: `guard`
(a `when` could not be evaluated, so the run stays), `moved` (the run had
already moved on, for example by a timer) or `settling` (the run moved,
but a later output step failed and is retried).

The panel also offers a Start button for each flow with an `on: manual`
start, and Cancel and Retry buttons when you hold that start's
permission. Retry tries the failed outputs of a step again now, instead of
waiting for the next attempt.

The sidebar has two pages when the project has flows. **Flow tasks**
lists the runs that wait for a role you hold, oldest first. **Flow runs**
lists every run you can read, newest first, filtered by flow and status.
Both lists read at most 500 (tasks) or 1,000 (runs) runs, the oldest
tasks and the newest runs; past that they say the list is cut short, and a
filter narrows it. The task list matches
each run against the current `flows.yaml`.

**Parallel branches.** A forked run shows each branch with its own state,
reason and step form. The Flow tasks page lists one task per branch that
waits for a role you hold. A branch that reached the join says it waits
for the others. The diagram marks the state of every branch, and the
timeline names the branch of each move.

**One step at a time.** A run counts its moves in a generation. The step
form sends the generation it was shown, and a step on a run that moved on
since is refused, so a stale form never moves a run twice. In a forked
run each branch has its own generation, under `parallel` in the run, so
one branch's step does not refuse another's. While one
person completes a step, the run is claimed for up to two minutes, and a
second completion is refused.

**Over HTTP.** The web app uses these routes; other clients may too.

| Route | What it does |
| ----- | ------------ |
| `GET /api/v1/_flows` | The flows of `flows.yaml`, with their diagrams |
| `GET /api/v1/_flows/tasks?flow=&page=&per_page=` | Your tasks |
| `GET /api/v1/_flows/runs?flow=&status=&page=&per_page=` | The runs you can read |
| `GET /api/v1/_flows/runs?subject=PER-12` | The runs on one entity, with why each waits |
| `POST /api/v1/_flows/runs` | Start a run: `{"flow":"onboarding","subject":"PER-12"}` |
| `GET /api/v1/_flows/runs/{id}` | One run, with why it waits and `_actions` |
| `POST /api/v1/_flows/runs/{id}/_complete` | Do the step: `{"generation":3,"properties":{"documents":true}}` |
| `POST /api/v1/_flows/runs/{id}/_cancel` | Cancel the run |
| `POST /api/v1/_flows/runs/{id}/_retry` | Retry the failed outputs now |

`_actions` on a run says whether `complete`, `cancel` and `retry` would
be accepted for you, by the same checks the POST routes make. In a forked
run, `complete` is false and each entry of `parallel` has its own
`complete`; send that entry's `generation` to complete its step. A run you
cannot read answers 404, like a missing entity. A missing role or
permission answers 403, a run that moved on or is claimed 409, and a
property outside `writes:` 422. Without flows, every route answers 404.

**Over MCP.** `list_flow_tasks`, `show_flow_run` and
`complete_flow_step` do the same for an agent. See
[MCP server](mcp-server.md#flow-tools).

## Edits outside a flow

By default a run does not stop anyone from editing its subject. Someone
with update rights can change `documents` in the entity form while the
onboarding run waits for HR to do that step. `outside_edits:` keeps such
properties for the flow:

```yaml
flows:
  onboarding:
    # ...
    outside_edits:
      default: allow
      properties:
        salary: { permission: onboarding.start }
      states:
        collect:
          properties:
            documents: deny
            stage: deny
```

Each rule is one of:

| Rule | Meaning |
| ---- | ------- |
| `allow` | Anyone with update rights may change the property. |
| `deny` | Nobody may change the property outside the flow. |
| `{permission: P}` | Only people who hold permission `P` may change it, globally or through a role relation on the subject. |

`properties:` gives a rule per property, and `default:` the rule for
every other property. Under `states:`, a state gives its own `default:`
and `properties:`. rela picks the first rule it finds, in this order: the
state's rule for the property, the state's `default`, the flow's rule for
the property, the flow's `default`, and otherwise `allow`. So in the
example, `documents` and `stage` are locked only while a run is in
`collect`, and `salary` is kept to holders of `onboarding.start` in every
state.

The rules apply to a subject while it has a run that has not ended. A
finished, failed or cancelled run locks nothing. When a subject has runs
of several flows, every run's rules apply: a property is writable only if
each run allows it. A subject keeps its id while it has a run that has
not ended: rela refuses to rename it, or to convert it to another type,
until the run ends or is cancelled.

The rules in force are always those of the current `flows.yaml`, also for
a run that started under an older version of the flow. An
`outside_edits:` block with no rules is a warning and locks nothing.

A run's own writes are not edits outside the flow. The values a person
writes when doing a step, and the run's `set` outputs, are not limited
by the rules of the state they come from. They are still limited by
every other run on the subject. While a run is forked, the rules of every
branch's state apply, so a step in one branch is still limited by the
rules of the other branches' states.

**Where the rules hold.** They apply wherever rela checks field rules on
a write:

- the web app and the data-entry API (`PATCH`, attachment uploads and
  restoring a past version): a locked
  property shows as read-only, `_transitions` lists its transitions as
  not allowed with reason `read-only`, and a write is refused with 403 and
  `field-affordance:read-only:<property>`;
- MCP on `rela-server`, scheduled Lua scripts, and channel promotes and
  outputs;
- the steps and outputs of other flows.

They do not apply to the CLI, MCP over stdio, `_actions` declared by the
operator, automations, `rela.bypass_acl`, declarative webhooks, or
CalDAV clients. These act for the operator or the system and do not
check field rules. The rules also do not cover the entity body,
relations or deletes, and do not limit creating a new entity. The
refusal to rename or convert a subject with an open run is not a field
rule: it holds on every surface, the CLI included, and rela records each
refused rename or conversion in the audit log. On the file backend without a
cache directory, runs are kept in the process that started them, so
another process does not see them and does not refuse the rename.

If rela cannot read the runs of a subject, every property of that subject
is read-only until it can.

On PostgreSQL and SQLite, rela checks the rules again inside the
transaction that writes the change. This holds for every surface above,
including a `PATCH`, an attachment upload and a restore in the web app. A
run that starts or moves to another state takes the same lock, so it
either lands before the check, which then sees it, or waits until the
write is done. The same lock orders a rename against a run start.

On the file backend there is no such lock, by decision: a run that starts
between the check and the write does not stop that one write. Resolving a
merge conflict, which exists only on the file backend, writes the file
directly and has the same gap. This is an accepted risk, tracked in
TKT-LU8PSKJ. Use PostgreSQL or SQLite where it matters.

**Stuck runs.** A run that cannot advance keeps its subject locked: a run
that failed to store its working state, a run whose output keeps failing,
or a run whose step needs a property that another run on the subject
denies. Each of these is listed by `rela flow runs`. `rela flow cancel`
ends the run and lifts its rules.

**Rules for the file.**

- `outside_edits:` needs `acl.yaml`. Without it no surface checks field
  rules, so rela refuses to load the flow.
- A state under `states:` must exist and may not be final. A state may
  not have its own `states:`.
- A property must be declared on the subject type, and may not be
  derived. Each level names at most 64 properties.
- A `permission:` that no role in `acl.yaml` grants is a warning.
- `RELA_AFFORDANCE_PROFILE=none` or `demo` is refused when a flow has
  `outside_edits:`, as it is for `fields:` rules.

A merge conflict on a locked property cannot be resolved through the API
while the run holds it. Resolve it after the run moves on, or cancel the
run.

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
