-- pgstore schema, version 17: scheduler task state and runs (BUG-TKL08E).
--
-- Backs internal/schedulerstate/pgschedstate. The scheduler used to keep its
-- bookkeeping in one state_kv document, rewritten whole on every update, and to
-- learn a run's outcome from an in-process channel. Several rela-server
-- processes against one database clobbered each other's document, and a lost
-- channel message stalled the scheduler. One row per task and one row per run
-- make both writes local and let any process see what is in flight.
--
-- Rows live in the tenant's schema like every other table here, so a
-- schema-per-tenant deployment scopes its schedule for free.
--
-- # Not graph content
--
-- A last-run stamp is a fact about the deployment's operation, not about an
-- entity. Keeping it out of the graph keeps a one-minute task from filling the
-- audit log and the version sweep. See the internal/schedulerstate package doc.

CREATE TABLE scheduler_tasks (
    task       text        PRIMARY KEY,
    -- NULL means "never succeeded" / "no retry pending".
    last_run   timestamptz,
    failures   integer     NOT NULL DEFAULT 0,
    next_retry timestamptz,
    -- Changes on every outcome; run creation is conditional on it.
    version    bigint      NOT NULL DEFAULT 0,
    touched_at timestamptz NOT NULL
);

CREATE TABLE scheduler_runs (
    id               text        PRIMARY KEY,
    task             text        NOT NULL,
    status           text        NOT NULL
        CHECK (status IN ('queued', 'running', 'succeeded', 'failed', 'abandoned')),
    created_at       timestamptz NOT NULL,
    started_at       timestamptz,
    finished_at      timestamptz,
    lease_until      timestamptz NOT NULL,
    node             text        NOT NULL DEFAULT '',
    error            text        NOT NULL DEFAULT '',
    -- The calendar slot a for_each run serves; '' for a plain run.
    occurrence       text        NOT NULL DEFAULT '',
    children         integer     NOT NULL DEFAULT 0,
    children_settled integer     NOT NULL DEFAULT 0,
    children_failed  integer     NOT NULL DEFAULT 0
);

-- The scheduler's non-overlap rule: at most one active run per task, across
-- every process sharing this schema.
CREATE UNIQUE INDEX scheduler_runs_one_active
    ON scheduler_runs (task) WHERE status IN ('queued', 'running');

-- The reaper's scan: active runs by lease expiry.
CREATE INDEX scheduler_runs_active_lease
    ON scheduler_runs (lease_until) WHERE status IN ('queued', 'running');

-- Prune's scan and a task's recent history.
CREATE INDEX scheduler_runs_task_finished ON scheduler_runs (task, finished_at);

-- A retried fan-out's lookup of the subjects already delivered.
CREATE INDEX scheduler_runs_occurrence ON scheduler_runs (task, occurrence) WHERE occurrence <> '';

-- One row per for_each subject of a run. The primary key is what makes a
-- redelivered child settle at most once.
CREATE TABLE scheduler_run_children (
    run_id  text    NOT NULL REFERENCES scheduler_runs (id) ON DELETE CASCADE,
    subject text    NOT NULL,
    settled boolean NOT NULL DEFAULT false,
    error   text    NOT NULL DEFAULT '',
    PRIMARY KEY (run_id, subject)
);
