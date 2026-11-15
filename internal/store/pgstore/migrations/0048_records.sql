-- pgstore schema, version 47: the record store (TKT-J7Q9C4K).
--
-- records holds typed rows owned by an entity: time series samples and
-- ledger postings. The row's fields are the doc column. record_groups holds
-- the ledger transaction ids claimed so far, so a transaction id is used
-- once and a reversal happens at most once. Times are BIGINT microseconds
-- since the epoch.

CREATE TABLE IF NOT EXISTS records (
	id          TEXT COLLATE "C" NOT NULL PRIMARY KEY,
	type        TEXT COLLATE "C" NOT NULL,
	owner       TEXT COLLATE "C" NOT NULL,
	ts          BIGINT NOT NULL,
	recorded_at BIGINT NOT NULL,
	recorded_by TEXT NOT NULL DEFAULT '',
	grp         TEXT COLLATE "C" NOT NULL DEFAULT '',
	reverses    TEXT COLLATE "C" NOT NULL DEFAULT '',
	doc         TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS records_owner_ts_idx ON records (type, owner, ts, id);
CREATE INDEX IF NOT EXISTS records_ts_idx ON records (type, ts, id);
CREATE INDEX IF NOT EXISTS records_grp_idx ON records (type, grp) WHERE grp <> '';
CREATE INDEX IF NOT EXISTS records_by_owner_idx ON records (owner);

CREATE TABLE IF NOT EXISTS record_groups (
	type TEXT COLLATE "C" NOT NULL,
	grp  TEXT COLLATE "C" NOT NULL,
	PRIMARY KEY (type, grp)
);
