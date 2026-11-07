-- pgstore schema, version 42: the flow run store (TKT-PV9R0GK).
--
-- flow_runs holds the execution state of each running flow, keyed by run
-- id; a row is deleted when its run completes. flow_definitions holds the
-- definition each version of a flow ran under, stored at the first run of
-- that version. See the sqlflowrun package doc.

CREATE TABLE IF NOT EXISTS flow_definitions (
	flow     TEXT COLLATE "C" NOT NULL,
	version  TEXT COLLATE "C" NOT NULL,
	def      BYTEA NOT NULL,
	digest   TEXT NOT NULL,
	saved_at BIGINT NOT NULL,
	PRIMARY KEY (flow, version)
);

CREATE TABLE IF NOT EXISTS flow_runs (
	id           TEXT COLLATE "C" NOT NULL PRIMARY KEY,
	flow         TEXT COLLATE "C" NOT NULL,
	version      TEXT COLLATE "C" NOT NULL,
	subject_id   TEXT COLLATE "C" NOT NULL,
	subject_face TEXT COLLATE "C" NOT NULL DEFAULT '',
	singleton    INTEGER NOT NULL,
	next_due_at  BIGINT NOT NULL DEFAULT 0,
	created_at   BIGINT NOT NULL,
	updated_at   BIGINT NOT NULL,
	rev          BIGINT NOT NULL,
	doc          TEXT NOT NULL,
	FOREIGN KEY (flow, version) REFERENCES flow_definitions (flow, version)
);

CREATE INDEX IF NOT EXISTS flow_runs_due_idx ON flow_runs (next_due_at, id) WHERE next_due_at > 0;
CREATE INDEX IF NOT EXISTS flow_runs_subject_idx ON flow_runs (subject_id, id);
CREATE INDEX IF NOT EXISTS flow_runs_flow_idx ON flow_runs (flow, id);
CREATE INDEX IF NOT EXISTS flow_runs_definition_idx ON flow_runs (flow, version);
-- A backstop to the subject lock: one singleton run per flow and subject id.
CREATE UNIQUE INDEX IF NOT EXISTS flow_runs_singleton_key ON flow_runs (flow, subject_id) WHERE singleton = 1;
