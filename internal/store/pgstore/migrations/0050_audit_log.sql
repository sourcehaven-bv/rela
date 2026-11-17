-- pgstore schema, version 49: the audit log (TKT-USQBMG6).
--
-- audit_log holds one row per audit record, hash-chained by seq.
-- audit_subjects indexes the entity ids each row names. audit_anchor holds
-- the seq and hash of the last pruned row. See the sqlaudit package doc.

CREATE TABLE IF NOT EXISTS audit_log (
	seq            BIGINT NOT NULL PRIMARY KEY,
	at             BIGINT NOT NULL,
	op             TEXT COLLATE "C" NOT NULL,
	principal_user TEXT COLLATE "C" NOT NULL,
	doc            TEXT NOT NULL,
	prev_hash      TEXT NOT NULL,
	hash           TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS audit_log_at_idx ON audit_log (at, seq);

CREATE TABLE IF NOT EXISTS audit_subjects (
	entity_id TEXT COLLATE "C" NOT NULL,
	seq       BIGINT NOT NULL,
	PRIMARY KEY (entity_id, seq)
);

CREATE INDEX IF NOT EXISTS audit_subjects_seq_idx ON audit_subjects (seq);

CREATE TABLE IF NOT EXISTS audit_anchor (
	id        INTEGER NOT NULL PRIMARY KEY CHECK (id = 1),
	seq       BIGINT NOT NULL,
	hash      TEXT NOT NULL,
	pruned_at BIGINT NOT NULL
);
