<!-- This file is auto-generated from docs-project/entities/. Do not edit directly. -->

# Flows (flows.yaml)

`flows.yaml` declares the flows of a project. A flow is a process that
follows one entity, its subject, through a series of states. An
onboarding flow, for example, follows a person from "collect documents"
to "finished", and creates a task on the way.

The same file declares the project's [triggers](#triggers): rules that set
values, create entities, notify people or run scripts when an entity
changes.

The file is optional. Without it, a project has no flows and no triggers.
`schema.yaml` never refers to it.

> **Status.** This release runs flows that a person starts by hand, from
> the web app, over MCP or with `rela flow start`, and flows that start
> when their subject is created (`on: create`). A run writes its
> outputs, follows `done` branches and ends in a final state. People do
> their steps in the web app or over MCP, and a completed step takes the
> `change` branches. Timers fire and failed steps are tried again while
> `rela-server` or the desktop app runs. The `predicate` and
> `schedule` triggers, `change` events from ordinary edits, retention and
> the inbox are checked but not acted on yet; they follow in a later
> release. See [Running flows](#running-flows) and
> [Doing a step](#doing-a-step).

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

### Starting on create

A flow with an `on: create` start gets a run each time an entity of its
subject type is created and the `when:` condition, if any, holds for the
new entity. Several `on: create` starts on one flow still give one run.
It works for every create that goes through rela's write path: the web
app and the API, MCP, `rela create`, `rela import --mapping`, a Lua
script (an action, a webhook action, a scheduled script) and a trigger's
`create` output.

- The run is started by the flow's `run_as`, the identity that writes
  its outputs. The person or script that created the entity needs no
  permission on the flow. So anyone who may create the subject makes
  `run_as` write. Grant `run_as` only the writes the outputs make, and
  use `when:` to narrow which creates start a run.
- On SQLite and PostgreSQL the run is stored in the same transaction as
  the entity. If the create fails or rolls back, no run exists. Because
  the run is part of the create, a retried request that creates nothing
  new starts nothing new either.
- On the file store nothing rolls back. A create that fails after the
  entity was written keeps the entity, and keeps its run too.
- Right after the create commits, and after the entity's own on-create
  automations, rela enters the initial state and writes its outputs. The
  request that made the create waits for this, for at most 30 seconds.
  If it fails or takes longer, the run waits with its step pending, and
  the flow worker tries again as for any failed step.
- With `per_subject: one`, a subject that already carries an unfinished
  run gets no second one. rela logs this and the create still succeeds.
- A `when:` reads only the new entity's own properties. A `when:` that
  cannot be evaluated starts no run. rela logs it and the create still
  succeeds.

Some ways of adding entities are not creates and start no run:
`rela import` of an export without a mapping, `rela db load`, a
conversion, a copy to another face and a restore from history.

A run's outputs can create entities that start further runs. Each run
started this way stores its `depth` on the run: one more than the run
whose output made the create. rela starts no run at depth 8 or deeper,
and logs it. The bound holds whether a run settles right after its
create or later through the flow worker. It limits the length of a
chain, not its width: one run that creates several subjects starts a run
for each.

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
| `job` | Work rela runs through the job queue: a script, an action or a webhook. See [Job steps](#job-steps). |
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
Timers fire while `rela-server` or the desktop app runs; see
[Running flows](#running-flows).

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
subject. Like a trigger that creates an entity, this is a choice the
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

A `when:` join is tested when a branch arrives. While some branches have
arrived and the condition is false, it is tested again every 5 minutes,
so a later change to the subject lets the run go on.

rela refuses a fork it cannot follow:

- every branch reaches exactly one join, the same one for the whole fork,
  and a join closes one fork;
- branches share no state, and no state outside a branch moves into one;
- a branch does not lead back to the state that forked, to the initial
  state, or to another fork: forks do not nest;
- from every state in a branch, the run can reach the join or a final
  state.

### Job steps

A state with `job:` runs one piece of work once its outputs are written,
and follows its `done` branch when the work succeeds.

```yaml
notify:
  job:
    kind: webhook
    url: https://hooks.example.com/onboarding
    secret: onboarding_hook
  on:
    done: { to: waiting }
provision:
  job:
    script: flows/provision.lua
    capabilities: { http: true, secrets: [directory_token] }
    retry: { attempts: 5, backoff: { initial: 30s, factor: 2, max: 10m }, timeout: 2m }
  on:
    done: { to: provisioned }
```

A job names exactly one target:

| Key | Runs |
| --- | ---- |
| `script` | A Lua script under `scripts/`. `capabilities` grants it http, ai, mail, write_file, tokens and named secrets, as for a trigger script; without it the script gets none. |
| `action` | An action from `data-entry.yaml`. It must be a plain script action or an `llm:` action with `apply: auto`. rela refuses a script action with `set`, `request`, `available_on`, `allow_acl_bypass` or `elevated_updates`, an `llm:` action that only proposes, and an `llm:` action whose `available_on` type is not the flow's subject type. `run_as` must hold the action's `permission`. The action's own `capabilities` apply. An `llm:` action writes its proposal as `run_as`; a refused run fails the step without a retry. |
| `kind: webhook` | A `POST` to `url` (http or https). `flows.yaml` is usually public, so keep tokens out of the URL and sign with `secret:` instead. |

An `llm:` action in a step reads text that people other than `run_as` may
have written, and that text can steer what the model proposes (prompt
injection). The proposal is then written with `run_as`'s grants. Grant
`run_as` only the writes the action's outputs make, and prefer outputs whose
values the schema bounds, such as enums and candidate relations. The model
can also copy anything `run_as` reads into what it writes, so `run_as` should
read no more than the people who can read the action's outputs. See
"Applying without review" in the data-entry guide.

The work runs as the flow's `run_as`, through the job queue. A script or
action sees the subject as `entity`, as `run_as` reads it, and
`rela.params` with `flow`, `flow_run`, `flow_state`, `attempt`,
`idempotency_key` and `step_key`.

A webhook posts the ids only: the flow, the run, the state, the subject,
the attempt and both keys. The receiver reads the subject through the API
with its own credentials. The request carries `Idempotency-Key` (the
attempt's key) and `X-Rela-Step` (the step's key). With `secret:`, it also
carries `X-Rela-Timestamp` and `X-Rela-Signature: sha256=<hex>`, the
HMAC-SHA256 of `<timestamp>.<body>` under the global secret of that name
in `.rela/secrets.yaml`. Check the signature and refuse an old timestamp.
rela does not follow a redirect.

A job state has `on: { done: ... }` and nothing else: no `waiting_for`,
`writes`, `after`, other events, and it may not be final. Its last `done`
branch has no `when`, so the run always has a way out.

**Retries.** rela tries the work again after an error that another
attempt may fix:

- a webhook answer of 429 or 5xx, or a network error;
- a script or action error, unless the script raised a table with
  `retry = false`: `error({ message = "unknown account", retry = false })`;
- an attempt the queue never started. This one does not count against
  `attempts`.

Any other webhook answer (a 3xx or another 4xx) fails the job at once.

| `retry` key | Meaning | Default |
| ----------- | ------- | ------- |
| `attempts` | Every execution, the first included: 1 to 10. | 3 |
| `backoff.initial` | The wait before the second attempt. | 10s |
| `backoff.factor` | What each next wait is multiplied by. | 2 |
| `backoff.max` | The longest wait, at most 1h. | 5m |
| `timeout` | How long one attempt may take once it starts: 1s to 10m. An attempt that does not start within this time is queued again. | 5m |

`retry: false` fails the job on the first error.

**Unknown outcomes.** When an attempt times out, or the process running it
stops, rela cannot tell whether the work happened. It does not try again:
the job fails with `timeout` or `lost`. Set `idempotent: true` when the
target is safe to run twice, and rela then tries again. The
`idempotency_key` (run, step and attempt) and the `step_key` (run and step,
the same for every attempt) let a receiver drop a repeat.

**A failed job** leaves the run in its state, with status `failed` in the
run's view and the reason: `exhausted`, `permanent`, `timeout` or `lost`.
In a fork, the other branches go on. A branch whose job waits or failed
has not arrived: a join that needs it waits, and one that does not (`any`,
a `when:` that holds, or another branch that reaches a final state) goes
on without it. Retry the run
(`Retry` on the run, as for a failed step) to start the job again with a
fresh count of attempts.

Jobs run in `rela-server` and the desktop app. A CLI command or an MCP
session over stdio does not run them: a job step waits until a server
runs it. On the filesystem backend and in the desktop app, the queue is
kept in memory, so an attempt queued when the process stops never starts,
and rela queues it again.

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

A property is restricted when some reader of the subject may not see it:

- a role's `visible:` block for the type leaves it out, or lists it only
  with a `when:`;
- a client baseline leaves it out of its `visible:` or names it in its
  `redact:`.

A role without a `visible:` block for the type sees every property. So a
condition may read a property that every role's `visible:` block for the
type lists without a `when:`. A block may still leave other properties
out. For example, the signup flow below reads `terms_accepted`, and the
customer role hides the account's internal ids:

```yaml
# acl.yaml
roles:
  staff:
    read: ["*"]
  customer:
    read: [account]
    visible:
      account:
        - field: name
        - field: terms_accepted
```

```yaml
# flows.yaml
flows:
  signup:
    subject: account
    states:
      details:
        on:
          change:
            - when: "entity.terms_accepted == true"
              to: accepted
```

The same rule applies to a template's `{{subject.<property>}}`.

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
active state, its start, completion and deadline times and, for a run
started on create, its depth (see [Starting on create](#starting-on-create)). Two relations
link it to the graph: `_carries` points at the subject, and `_produced`
points at each entity a step created.

**Who can read a run.** A run is readable exactly when its subject is.
rela follows the run's `_carries` edge and applies the subject's read
rules, including role relations and client ceilings. A run whose subject
you cannot read answers like a missing entity. A run with no subject,
or whose subject was deleted, is hidden from everyone but the engine. A
role relation edge on the subject itself counts, also when acl.yaml
declares `inherit_roles_through`. Some subjects do not show their runs
yet: a subject you read only through `inherit_roles_through`, through a
face-split role relation, or through one of several role relations when
one of them has `when:`. `rela acl who-can read` on a run lists only the
routes that show the run.

**Who can write a run.** Only the flow engine, as `system:flow`. acl.yaml
may not name `_flow-run`, `_carries`, `_produced`, a role starting with
`_`, or `system:flow`, and `"*"` in a grant does not cover them. The rule
also holds for `rela.bypass_acl(...)`, for a project without acl.yaml,
and for `--run-as`, which refuses `system:flow`. Nobody renames or
converts a run. Triggers and validation rules do not fire on runs;
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

A flow with an `on: create` start also starts by itself; see
[Starting on create](#starting-on-create).

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

**The flow worker** fires due timers, tries failed steps again and tests
waiting `when:` joins. It runs inside `rela-server` and the desktop app,
every 15 seconds. A CLI command or an MCP session over stdio does not run
it: without a server, a timer fires and a failed step is tried again once
a server starts. With several server processes on one project, one of
them runs the worker; on PostgreSQL each tenant has its own. In
multi-tenant `rela-server`, a tenant's worker runs while the tenant is
loaded. One pass handles at most 256 runs; the rest wait for the next
pass. On PostgreSQL the worker keeps one database connection while it
runs, so `pool_max_conns` must be at least 4; `rela-server` refuses to
start with a smaller pool when the project declares flows.

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

A next-action source can put your oldest task in a banner on every
page, with a button to its step: see `source: flow_tasks` in
[Next actions](data-entry.md#waiting-flow-steps-source-flow_tasks). An
entity page link with `?run=<id>` scrolls to that run and focuses it.

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
operator, triggers, `rela.bypass_acl`, declarative webhooks, or
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

## Triggers

A trigger is a rule that runs when an entity or relation changes. It can set
a property, link or create an entity, put the entity on a pile, notify
people, run a script, or check the entity and warn. Triggers live in
`flows.yaml` under the top-level `triggers:` key, beside `flows:`. A file may
hold only triggers, only flows, or both.

Before this release, triggers were declared in `schema.yaml` under
`automations:`. Other guides and the audit log still call them automations.
See [Moving from schema.yaml automations](#moving-from-schemayaml-automations).

```yaml
triggers:
  ticket-planning-checklist:
    description: Create a planning checklist when a ticket enters planning
    on:
      entity: ticket
      property: status
      becomes: planning
    outputs:
      - create:
          type: planning-checklist
          properties:
            title: "Planning: {{new.title}}"
            status: in-progress
          relate: { type: has-planning, from: subject }
          if_exists: skip
```

`triggers:` maps a name to a rule. A rule has three keys:

| Key | Meaning |
| --- | ------- |
| `description` | Optional text that says what the rule is for. |
| `on` | When the rule fires. See [Trigger events](#trigger-events). |
| `outputs` | What the rule does, in order. See [Trigger outputs](#trigger-outputs). |

A name may not be empty and may not contain `.` or `:`. Each name is used
once. The audit log records every write a rule makes as
`triggered_by: automation:<name>`, and a waiting background job finds its
rule by name. So renaming a rule changes its audit label, and a job that
waits under the old name ends without running.

### Trigger events

The keys under `on:` decide which changes fire the rule.

| Key | Meaning | Example |
| --- | ------- | ------- |
| `entity` | The entity types to watch: a name or a list. An abstract type stands for all its variants. Without it, the rule watches every type. | `[ticket, bug]` |
| `property` | Fires on an update that changes this property. Does not fire on a create. | `status` |
| `becomes` | With `property`: only when the new value is this value. | `in-progress` |
| `from` | With `property`: only when the old value was this value. | `backlog` |
| `created` | Fires when an entity is created. | `true` |
| `updated` | Fires on an update that changes a property or the body. Does not fire on a create. Not with `property`, `becomes` or `from`. A script under it must be a background script. | `true` |
| `deleted` | Fires once for each face a delete removes. Stands alone, and every output must be a background script. See [Running a script when an entity is deleted](#running-a-script-when-an-entity-is-deleted). | `true` |
| `relation_created` | Fires when a relation of this type is created. | `implements` |
| `relation_removed` | Fires when a relation of this type is removed. | `implements` |
| `faces` | The content states to watch: a name or a list. Without it, the rule watches every state. See [Scoping a trigger to content states](#scoping-a-trigger-to-content-states). | `[published]` |
| `copy` | The copy definitions to watch: a name or a list. See [Triggers on a copy](#triggers-on-a-copy). | `adopt` |
| `where` | A list of filter clauses that must all hold. See [Trigger conditions](#trigger-conditions). | `["kind=enhancement"]` |
| `when` | A predicate expression that must hold. See [Trigger conditions](#trigger-conditions). | `"entity.priority == 'high'"` |

The event keys are `created`, `updated`, `property`, `deleted`,
`relation_created`, `relation_removed` and `copy`. They are alternatives: a
rule with `created: true` and `updated: true` fires on both. The other keys
narrow the rule. A rule without an event key never fires.

### Scoping a trigger to content states

A type that declares `faces:` stores each content state as a separate row
(see [Content states](content-states.md)). A rule fires on every one of them
by default. A ticket in translation is still a ticket, so "when a ticket's
status becomes done" applies to each state.

This can multiply side effects. A rule that creates a checklist creates one
for each state, which is rarely what you want. Use `faces:` when the rule is
about particular states:

```yaml
triggers:
  announce-publication:
    on:
      entity: policy
      property: status
      becomes: approved
      faces: [published]
    outputs:
      - set: { properties: { announced_at: "{{today}}" } }
```

Name the faces as the type declares them. A face that the watched types do
not declare is a load error, because the rule would never fire.

Outputs write back to the state that fired the rule. A rule that fires on a
translation updates that translation, not its sibling.

### Triggers on a copy

A [copy](metamodel.md#declaring-copies) runs the triggers of the face it
writes. When the copy creates the face, the `created` rules run. When it
overwrites an existing face, the `updated` rules run, with the face as it was
before the copy as the old state.

The `set` outputs are part of the copy's own write, as on an ordinary
update. The copy stays one write with one audit record. A value a rule sets
must make a legal state-machine move from the value the copy wrote, or the
copy is refused and nothing is written. The other outputs, such as `create`
and scripts, run after the copy is written. If one of them fails, the copy
stays written and the response carries a `follow_up_failed` warning.

This applies to every `created` and `updated` rule on the target type, not
only to rules that name a copy. Use `faces:` to keep a rule off the face a
copy writes.

Use `copy:` to run a rule only when a particular copy writes the face. On its
own, `copy:` fires on every run of the copy, also when the copy changes no
value:

```yaml
triggers:
  stamp-adoption:
    on:
      entity: policy
      copy: adopt
    outputs:
      - set: { properties: { review_on: "{{today}}" } }
      - script:
          file: publish-site.lua
          background: true
```

Beside `created`, `updated` or `property`, `copy:` limits that key to writes
by the named copies. Without `copy:`, a rule fires on a copy's write like on
any other write.

rela refuses a `copy:` that names no declared copy, that writes a type or
face which `entity:` or `faces:` leaves out, or that is combined with
`relation_created` or `relation_removed`. Each of these would never fire.

A rename re-runs a copy rule's background scripts only on a face that one of
its copies writes.

To stamp a value on every run of one copy, the copy's own `set:` key is
simpler than a trigger.

### Trigger conditions

Two keys add conditions. Both are optional. When both are set, both must
hold. rela checks them against the entity's new state, after the change. To
test the old value of the changed property, use `from`.

`where:` takes a list of filter clauses, in the same syntax as validation
rules. All clauses must match:

```yaml
triggers:
  mark-enhancement-for-docs:
    on:
      entity: ticket
      property: status
      becomes: review
      where:
        - "kind=enhancement"
        - "priority=high"
    outputs:
      - set: { properties: { needs_docs: "true" } }
```

The operators are `=`, `!=`, `<`, `<=`, `>`, `>=` and `=~` (regular
expression).

`when:` takes a predicate expression, the language of the CLI's `--filter`
flag and of ACL rules. It adds `and`, `or`, `not` and functions, such as
date arithmetic:

```yaml
triggers:
  flag-due-soon:
    on:
      entity: taak
      created: true
      where: ["status=todo"]
      when: "days_between(entity.due, today()) <= 7"
    outputs:
      - set: { properties: { flag: due-soon } }
```

The functions include `today()`, `now()`, `days_between(a, b)`,
`date_add(d, n, unit)`, `rrule_next(rule, after)`, and the string matchers
`match`, `regex`, `fuzzy` and `contains`. An expression can choose a value
with `c and x or y`; see
[Choosing a value](metamodel.md#choosing-a-value-with-andor). A date literal
in an expression is a string: `entity.due <= '2026-08-25'`.

`when:` needs `entity:`, because rela compiles the expression against the
properties of each named type. `related(...)` (see
[Filtering on related entities](metamodel.md#filtering-on-related-entities))
works in `when:`, but not on a `created` or `deleted` rule. On a create the
relations are written after the entity, and on a delete they are gone. It
counts every related entity, whoever made the write. If rela cannot answer a
`related(...)`, the rule does not fire.

The keys are separate because the two syntaxes overlap without an error.
The filter parser would read `days_between(entity.due, today()) <= 7` as a
filter on a property named `days_between(entity.due, today())`. That filter
matches nothing and reports nothing.

A `where:` clause that does not parse, or a `when:` that does not compile, is
a load error that names the rule. A dropped condition would make the rule
fire on more entities than you wrote.

#### Guard optional date properties

A date function on a property the entity does not have is an evaluation
error, not `false`. The rule does not fire, and the write result carries a
warning:

```yaml
when: "days_between(entity.due, today()) <= 7" # skips tasks without a due date
```

A task without a due date may be the one you want to flag. Guard the
property when it is optional:

```yaml
when: "entity.due ~= nil and days_between(entity.due, today()) <= 7"
```

Or test for the missing value:

```yaml
when: "entity.due == nil or days_between(entity.due, today()) <= 7"
```

### Trigger outputs

`outputs:` is a list. Each entry holds exactly one of these keys:

| Output | What it does |
| ------ | ------------ |
| `set` | Sets properties on the entity that fired the rule. |
| `relate` | Creates a relation from that entity. |
| `create` | Creates a new entity, and optionally links it. |
| `add_to_pile` | Puts the entity on a user's pile. |
| `notify` | Puts the entity on a declared pile of other users. |
| `script` | Runs a Lua script, inline or in the background. |
| `check` | Checks the entity and reports a warning or an error. |

Values under `properties:` are YAML scalars. rela keeps their text, and they
may hold [placeholders](#template-placeholders).

#### set

```yaml
outputs:
  - set:
      properties:
        started_at: "{{today}}"
        reviewed: "false"
```

`set` changes the entity that fired the rule. The values are part of the
same write, so the save stores them with one audit record. `set` needs
`properties:`.

#### relate

```yaml
outputs:
  - relate: { type: implements, to: "{{new.parent}}" }
```

`relate` creates a relation of `type` from the entity to the id in `to`. When
`to` is empty after the placeholders are filled in, nothing happens.

#### create

```yaml
outputs:
  - create:
      type: checklist
      template: "{{new.kind}}"
      properties:
        title: "Planning: {{new.title}}"
        status: in-progress
      relate: { type: has-planning, from: subject }
      if_exists: skip
```

| Field | Required | Meaning |
| ----- | -------- | ------- |
| `type` | yes | The type to create. One concrete type, not an abstract one. |
| `template` | no | A template variant: `bug` loads `templates/entities/<type>--bug.md`. It may hold placeholders. After they are filled in, it may hold only letters, digits, `-` and `_`. |
| `properties` | no | The properties of the new entity. |
| `relate` | no | Links the subject to the new entity. `type` names the relation. `from: subject` is required: the relation goes from the entity that fired the rule to the new one. |
| `if_exists` | no | What to do when the subject already has that relation: `skip` (the default) creates nothing, `error` reports an error, `replace` deletes the existing entity and creates a new one. |

`if_exists: skip` keeps a rule from creating a second checklist when a ticket
enters the same state again.

#### add_to_pile

```yaml
outputs:
  - add_to_pile: { pile: Inbox, owner: "{{new.assignee}}" }
```

A pile is a personal, named stack of entities that one user collects and
works through. This output puts the entity on a pile. The face is kept, so
`POL-1@draft` lands on the pile as that face.

| Field | Required | Meaning |
| ----- | -------- | ------- |
| `pile` | yes | The pile name, or the key of a declared pile. May hold placeholders. |
| `owner` | no | The user whose pile receives the entity. May hold placeholders. |
| `create` | no | Create the pile when it does not exist. Defaults to `true`. |

- Without `owner`, the entity goes on a pile of the user who made the write.
- `owner` must be that user or an existing entity of the ACL's
  `user_entity_type` (a person). rela refuses anything else.
- When `owner` is set but is empty after the placeholders are filled in, for
  example on an unassigned ticket, nothing happens.
- A push to another user's pile is write-only. It never reveals whether that
  pile exists or how full it is.
- A push to another user's pile never removes their items. On a full pile
  the entity is not added. The push creates a pile only while that user has
  fewer than 25 piles, so 25 of the 50 are always left for the user. Each
  such push is logged with the pushing user, the target user and the number
  of entities, but not the pile name.
- The output is best-effort. A failed push is logged with the rule name and
  never fails the write. A pile is a place for notices, not a system of
  record.
- On PostgreSQL the push runs outside the entity's transaction. If the write
  then rolls back, the pile holds a reference to an entity that does not
  exist. Readers skip it, so it is harmless.

#### notify

```yaml
triggers:
  tell-project-owner:
    on:
      entity: task
      property: status
      becomes: done
    outputs:
      - notify:
          pile: inbox
          reason: done
          to: { via: part-of, property: owner }
```

This output puts the entity on a [declared pile](metamodel.md#declared-piles)
of other users. The common case is to tell the owner of a project that
someone completed one of its tasks.

| Field | Required | Meaning |
| ----- | -------- | ------- |
| `pile` | yes | The key of a declared pile with `read_state: true`. |
| `reason` | no | One of the pile's `reasons`. Users see its text next to the item. |
| `to.property` | yes | The property that holds the recipients: a person id, or a list of them. |
| `to.via` | no | A relation path from the entity to the entity that holds `to.property`: a name or a list. Without it, rela reads the property from the entity itself. |
| `to.type` | no | Limits the last step of `to.via` to one entity type. Needs `to.via`. |

A notification goes to another user, so `notify` needs person mapping:
`user_entity_type` and `principal_property` in `acl.yaml`. Without it, rela
sends no notification and logs a warning for each one.

- `to.via` is written like a traversal in `related(...)`: relation names,
  outgoing or by their inverse name. It needs `on.entity:`. rela resolves the
  path for each of those types at load. A path that does not resolve, or that
  ends on a type without `to.property`, is a load error.
- The user who made the write is never notified. A notification is about what
  others do.
- Each item records who made the write and the reason. The pile shows both,
  for example "Bob completed a task".
- rela finds the recipients in the stored graph, whatever the writing user
  may read. When a recipient opens the pile, each item is checked against
  that recipient's own access.
- A path reaches at most 1000 entities. rela skips further recipients.
- Like `add_to_pile`, the output is best-effort. A failure is logged with the
  rule name and the entity, never the recipient, and the write succeeds.

#### script

```yaml
outputs:
  - script:
      file: notify.lua
```

`script` runs Lua. It holds exactly one of `lua:` (inline code) and `file:`
(a path under `scripts/`). The script gets the saved entity as `entity` and
its previous state as `old_entity`. It runs in the request that saved the
entity, as the user who saved it. In inline code, rela fills in only the safe
placeholders, such as `{{today}}` and `{{user.name}}`. Entity values reach
the script as `entity`, never as source code.

| Field | Meaning |
| ----- | ------- |
| `lua` | Inline Lua code. |
| `file` | A script file under `scripts/`. |
| `capabilities` | What the script may reach: `http`, `ai`, `mail`, `write_file`, `secrets` and, for a background script, `tokens`. Without it the script gets none. See the [Lua scripting guide](lua-scripting.md). |
| `allow_acl_bypass` | `read`, `write` or `read+write`: unlocks `rela.bypass_acl`. Not on a background script. See [Elevated access](lua-scripting.md#elevated-access--relabypass_acl). |
| `background` | `true` runs the file as a background job. Needs `file:`. See [Background scripts](#background-scripts). |
| `run_as` | The identity a background job runs as. The default is `system:automation`. |
| `retry` | For a background job: `never`, `bounded` (the default) or `persistent`. |

#### check

```yaml
outputs:
  - check:
      where: "estimate>0"
      severity: warning
      message: "{{entity.id}} has no estimate"
```

`check` tests the entity against the filter clause in `where`. When the
entity does not match, rela adds `message` to the write result: as a warning,
or as an error with `severity: error`. The default is `warning`. A check
reports; it does not refuse or undo the write. `message` is required and may
hold placeholders. A clause that does not parse becomes a warning when the
rule fires. To refuse a write, use a validation rule with `enforce: write`
(see [Metamodel](metamodel.md#enforcing-a-rule-on-write-enforce-write)).

### Background scripts

`background: true` runs a script file as a background job, so a save does not
wait on it. Use it for a script that calls an outside system.

```yaml
triggers:
  push-to-tracker:
    on: { entity: ticket, created: true, updated: true }
    outputs:
      - script:
          file: tracker/push.lua
          background: true
          run_as: tracker-sync      # default: system:automation
          retry: bounded            # never | bounded (default) | persistent
          capabilities: { http: true, secrets: [tracker_token] }
```

A background script differs from an inline one in these ways:

- **Identity.** The job runs as `run_as`, or as `system:automation`, never as
  the user who saved. Reads, writes and transition guards are checked against
  that identity's grants in `acl.yaml`, field grants included. Give it a role
  there. Without one, the job cannot read the entity and ends with a warning
  in the server log. `run_as` lets any user who can save a matching entity
  start a script with that identity's access. Treat it as an elevation you
  grant, and treat the entity's values as untrusted input. `run_as` may not
  name another `system:` identity. The audit log records the job as
  `automation-job:<name>;by=<user who saved>`.
- **What the script sees.** The job reads the entity again when it runs, so
  `entity` is the current state, not the saved one, and `old_entity` is nil.
  rela does not check `where:` and `when:` again.
- **Coalescing.** Saves of one entity while its job waits make one run. A
  save while the job runs makes it run once more afterwards. A save that
  arrives just as a job finishes is queued again shortly after. A job stops
  rerunning after 5 runs and fails, so `retry:` decides whether it runs
  again.
- **Renames.** A job that waits for the old id finds nothing. So a rename
  checks the rule again for the new id, and starts its background scripts if
  it matches. Only background scripts run on a rename.
- **Failures.** A script error is retried as `retry:` says. A job can run a
  script more than once, so write it so that a second run does no harm. A
  job whose output was removed from `flows.yaml`, or whose entity was
  deleted, ends without running.
- **Loops.** The job's own writes do not start it again for the same entity.
  A chain of background jobs that start each other stops after 8 steps.
- **Where it runs.** `rela-server` and the desktop app run jobs on the job
  queue. Every other command, including `rela mcp` and `rela scheduler`, runs
  the script in the foreground, right after the save, because a one-shot
  command would exit before the queue ran the job. There `retry:` does not
  apply, and a failed script makes the save report only that the background
  script failed; the error goes to the log. On the file and SQLite backends
  the queue is in memory, so jobs that still wait when the process stops are
  lost.
- **State.** In queue mode each (rule, script, entity) keeps two small
  records in the project's state store (`.rela/automation-jobs/` on the file
  backend). They are not removed when the entity is deleted.

One rule may run a given file in the background only once, because a waiting
job finds its output by rule name and file.

### Running a script when an entity is deleted

`deleted: true` starts background scripts when an entity is deleted. Use it
to tell an outside system, such as a published site, that a record is gone.

```yaml
triggers:
  unpublish-page:
    on: { entity: pagina, deleted: true, faces: [published] }
    outputs:
      - script:
          file: site/unpublish.lua
          background: true
          capabilities: { http: true }
```

- **When it fires.** Once for each face the delete removed, after the delete
  is saved. A delete from the web app, the CLI, MCP, a script or another rule
  fires it. So does a soft delete, the kind the web app can undo. An owned
  entity deleted with its owner fires its own rules. Purging a soft-deleted
  entity later does not fire again, and neither does an undo or a restore.
  If the face is restored, or created again under the same id, before the
  job runs, the job does nothing.
- **What the script sees.** `entity` is the face as it was just before the
  delete. `where:` and `when:` decide on that state too. `when:` may not use
  `related(...)`, because the relations are deleted with the entity.
- **Identity.** The job runs as `run_as` or `system:automation`, as other
  background scripts do. It sees the deleted face only as far as that
  identity may read it. Fields its `visible:` grants hide are removed. A read
  grant that depends on the stored entity, such as a role given through a
  relation, cannot be checked once the entity is gone, so the job does not
  run. Give the job's identity a plain read grant on the type.
- **No coalescing.** Each delete is its own job. Deleting, recreating and
  deleting an entity again runs the script twice.
- **Retention of the deleted state.** In queue mode the job stores the
  deleted face encrypted, with hidden fields included. The key that decrypts
  it is a separate record in the state store, never in the job table. The
  job erases the key when it ends: after it succeeds, after it decides not
  to run, and after its last allowed attempt fails. A failure that will be
  retried keeps the key for the retry. Purging the entity makes the state of
  all its waiting jobs unreadable, and those jobs end without running. The
  purge of a soft-deleted entity does this, and so does
  `rela history-purge --commit`. A job the queue itself gives up on, for
  example one past the time limit of `retry: persistent`, keeps its key. On
  PostgreSQL the queue deletes finished and failed job rows after 24 hours,
  and the encrypted copy goes with them. On the file and SQLite backends the
  queue is in memory, so the encrypted copy is gone when the process stops.
  In foreground mode the state is not stored at all. This erases the state
  from the live database. Backups taken while a job waited keep both the key
  and the encrypted copy.
- **Allowed keys.** `deleted` cannot be combined with `created`, `updated`,
  `property`, `becomes`, `from`, `relation_created` or `relation_removed`.
  Every output must be a background script, and `check` is refused. The
  delete has already happened when the rule runs, so there is no save to
  change or warn about.

### Template placeholders

Output values may hold placeholders in `{{...}}`. rela fills them in when the
rule fires.

| Placeholder | Value |
| ----------- | ----- |
| `{{new.<property>}}` | A property of the entity after the change, such as `{{new.title}}`. |
| `{{old.<property>}}` | A property of the entity before the change. |
| `{{entity.id}}` | The entity's id. |
| `{{entity.type}}` | The entity's type. |
| `{{user.name}}` | The git `user.name` of the machine that runs rela, or `$USER` without one. |
| `{{user.email}}` | The git `user.email` of the machine that runs rela. |
| `{{today}}` | The current date, such as `2026-11-15`. |
| `{{now}}` | The current time in ISO 8601 format. |

Placeholders work in `set` values, `relate.to`, `create.template`,
`create.properties`, `add_to_pile.pile`, `add_to_pile.owner` and
`check.message`. Any other placeholder is a load error. Earlier releases
wrote an unknown placeholder into the data as literal text. The state outputs
of a flow use `{{subject.<property>}}`; a trigger does not.

### Order of evaluation

rela checks the rules in the order they appear in `flows.yaml`. Each rule
that fires adds its outputs in the order they are written.

The outputs do not all write at the same moment:

- `set` values are part of the save itself. When two rules set the same
  property, the later rule wins.
- Inline scripts, relations, pile pushes and notifications run after the
  save.
- New entities are created last. A created entity fires its own `created`
  rules. A chain of such writes stops after 50 steps with a warning.
- `check` outputs are judged after the other outputs of the rule.

If a `create` fails, for example because its template name is not allowed,
the write result carries an error. The other outputs still run.

### Load checks

rela checks the triggers when it loads the project, in `rela validate`, and
in the review step of the [Configure space](data-entry.md#configure-space).
A rule with a problem stops the load, and the error names the rule. rela
refuses:

- a name that is empty, holds `.` or `:`, or is used twice;
- a rule without an event key under `on:`, and a rule with neither outputs
  nor checks;
- YAML anchors and aliases anywhere under `triggers:`;
- an unknown key, or an output entry with more or fewer than one key;
- a `where:` clause that does not parse, or a `when:` that does not compile;
- a `when:` without `entity:`, and `related(...)` on a `created` or
  `deleted` rule;
- a face or copy the rule can never match;
- an entity type or relation that rela manages itself (names that start with
  `_`), and an abstract type as `create.type`;
- `updated` together with `property`, `becomes` or `from`, and an inline
  script under `updated`;
- `deleted` together with another event key, a `check` under it, or an
  output that is not a background script;
- `background: true` without `file:` or with `allow_acl_bypass`; `run_as`,
  `retry` or `capabilities.tokens` without `background: true`; a `run_as`
  that names another `system:` identity; one file run twice in the
  background by one rule;
- `script` without exactly one of `lua:` and `file:`, `set` without
  `properties:`, `create` without `type:`, `create.relate.from` other than
  `subject`, an unknown `if_exists`, and `check` without `message:` or with a
  `severity` other than `warning` or `error`;
- `add_to_pile` without `pile:`, and a `notify` whose pile is not declared,
  lacks `read_state: true` or does not know the `reason`, or whose path does
  not resolve;
- an unknown placeholder, including one with spaces inside the braces;
- a `set` or `create` that writes a file property or an external ref, and a
  `property` key that names an external ref. Only the attachments API changes
  a file, and only a script changes a ref.

A schema change can break a rule, for example by removing a property it
names. `rela validate` and the Configure review catch this before the change
is saved, and `rela flow validate` checks them too. `rela mcp` reloads `flows.yaml` when the file changes. If the new
file does not load, it keeps the previous configuration and logs a warning.

### Examples

Create a checklist each time a ticket moves to a new stage:

```yaml
triggers:
  ticket-planning-checklist:
    on: { entity: ticket, property: status, becomes: planning }
    outputs:
      - create:
          type: planning-checklist
          properties: { title: "Planning: {{new.title}}", status: in-progress }
          relate: { type: has-planning, from: subject }
          if_exists: skip
  ticket-review-checklist:
    on: { entity: [ticket, bug], property: status, becomes: review }
    outputs:
      - create:
          type: review-checklist
          properties: { title: "Review: {{new.title}}", status: in-progress }
          relate: { type: has-review, from: subject }
          if_exists: skip
```

Record when work started:

```yaml
triggers:
  track-started:
    on: { entity: [ticket, bug], property: status, becomes: in-progress }
    outputs:
      - set:
          properties:
            started_at: "{{today}}"
            started_by: "{{user.name}}"
```

Name each rule after what it does, keep each rule to one task, and add a
`description:`.

rela's own issue tracker runs its ticket workflow this way. Its triggers are
in `tickets/flows.yaml` in the rela repository, and
`tickets/tests/checklist-workflow.test.lua` tests each of them with
`rela test`.

A property trigger fires when the value changes, not when an entity is
created with that value. A ticket created with `status: planning` gets no
planning checklist. Create it in an earlier status and then move it.

### Moving from schema.yaml automations

rela no longer reads `automations:` from `schema.yaml`. A schema that still
declares them, in the root file or in an included file, does not load:

```text
automations: are declared in flows.yaml under triggers: since this release
(TKT-GWOBH2U); run 'rela migrate' to move them
```

Run `rela migrate` (short for `rela migrate config`) to move them. It adds
each automation to `flows.yaml` under `triggers:` and creates the file when
needed. It keeps comments and removes the automations from the schema files.
`rela migrate --check` reports a pending move without writing.

| `automations:` entry | `triggers:` rule |
| -------------------- | ---------------- |
| `name: x` | The key `x`, byte for byte. An entry without a name gets `automation-<n>`, with `n` its position. |
| `on.when: [...]` | `on.where: [...]` |
| `on.condition: x` | `on.when: x` |
| Other `on:` keys | The same key under `on:` |
| `do:` | `outputs:`. A `do:` item with several keys becomes several outputs, in the order `set`, `create_relation`, `add_to_pile`, `notify`, `create_entity`, `lua`, `lua_file`. |
| `set: p` and `value: v` | `set: { properties: { p: v } }` |
| `create_relation: { relation: r, to: t }` | `relate: { type: r, to: t }` |
| `create_entity: { ..., relation: r }` | `create: { ..., relate: { type: r, from: subject } }` |
| `lua: src` | `script: { lua: src }`, with the same `capabilities` and `allow_acl_bypass` |
| `lua_file: f` | `script: { file: f }`, with the same `capabilities`, `allow_acl_bypass`, `background`, `run_as` and `retry` |
| `add_to_pile`, `notify` | Unchanged |
| `validate: [{ check, severity, message }]` | A `check: { where: <check>, severity, message }` output at the end |

Because names are kept, audit records and waiting background jobs still
match their rule.

The move stops when one of the cases below applies. It then writes nothing
to `flows.yaml` and cuts nothing from the schema. Syntax migrations that
the same run applies to `schema.yaml` and to included files that declare
automations may already be written. The move stops when:

- an entry has a key the old loader did not know;
- an entry uses a YAML anchor or alias;
- one `do:` item holds `create_entity` together with `lua` or `lua_file`
  (split it into two items first);
- a name holds `.` or `:`, or is used twice;
- the automations are in an included file outside the project directory, or
  in an included file with `namespace:`;
- `flows.yaml` already has a trigger of the same name with different content.

The error names the file, the automation and the key.

The Configure space edits the moved triggers in `flows.yaml`. See
[Configure space](data-entry.md#configure-space).

## Checking the file

```bash
rela flow validate
rela flow show
rela flow show onboarding
```

`rela flow validate` loads `flows.yaml` against the schema and acl.yaml
and prints each problem with its key, such as
`flows.onboarding.states.review.on.done.to: names "finish", which states
does not declare`. `rela validate` runs the same check, and also checks the
[triggers](#load-checks). `rela flow show`
lists the flows, and with a name prints that flow as a Mermaid state
diagram.

`rela flow show` also lists the *headless flows*: the state machines that
`transitions:` declares on an enum type in `schema.yaml` (see
[Metamodel](metamodel.md#state-machines-transitions)). A headless flow has
no runs; its state is the property itself. It is named `schema.` followed
by the enum type's name, such as `schema.ticket-status`, and it lists the
properties that use the type instead of a subject. `rela flow show
schema.ticket-status` prints its diagram, and `rela flow validate` checks it
too. In JSON output, each flow has `kind`: `flow` for one from `flows.yaml`,
`headless` for one from the schema.

Names starting with `_` are reserved in `schema.yaml` for the types rela
adds for flows. See [Metamodel](metamodel.md).
