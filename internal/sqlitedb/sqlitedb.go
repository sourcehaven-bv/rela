// Package sqlitedb owns the SQLite database FILE: opening it, verifying its
// PRAGMAs, holding the single-writer lock, and carrying its schema forward.
//
// It exists because a rela database file holds two unrelated things — the
// entity graph and the operator's config — and neither should own the other.
// Both are handed the opened handle:
//
//	db, err := sqlitedb.Open(ctx, sqlitedb.Options{Path: p})  // owns the file
//	cfg     := configsql.New(db.DB())                          // reads config
//	st, err := sqlitestore.New(db)                             // reads the graph
//
// That ordering is also what config needs: the metamodel has to load before
// anything that consumes a metamodel, the store included. Previously the store
// owned opening, which made config a guest in a package whose job is the
// graph.
//
// # Findings carried from the spike (TKT-TWIO11) — do not rediscover
//
// Each of these was measured, and each is cheap to reintroduce by accident:
//
//   - PRAGMAs go in the DSN, never db.Exec. A PRAGMA is per-connection, so
//     db.Exec configures whichever pooled connection served it and leaves every
//     later one at the default — while reading back correctly. Measured:
//     busy_timeout set that way failed at 0.00s instead of waiting 5s.
//   - Transactions use BEGIN IMMEDIATE. A deferred transaction that reads then
//     writes must upgrade its lock mid-flight, and the upgrade cannot wait, so
//     it returns SQLITE_BUSY regardless of busy_timeout.
//   - Never serialize by shrinking the pool. MaxOpenConns(1) deadlocks: a
//     transaction pins the only connection while readers block waiting for one.
//     That is database/sql pool starvation, not a SQLite lock.
package sqlitedb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "modernc.org/sqlite" // registers the "sqlite" driver
)

// DB is an opened, verified SQLite database: the single-writer lock is
// held, the DSN PRAGMAs are confirmed to have reached more than one pooled
// connection, the schema exists and its version is stamped.
//
// It exists so config can be read BEFORE a store is built. A self-contained
// project keeps its schema.yaml and the rest of its operator-authored config
// in the same database as the data, and the metamodel must be loaded before
// anything that consumes a metamodel — the store included. Splitting the
// connection from the store makes that ordering expressible:
//
//	conn, err := sqlitestore.Connect(ctx, opts)   // database usable
//	meta, err := loadMetamodel(conn)              // config read from it
//	st, err := sqlitestore.New(conn, …)           // store built on it
//
// This mirrors pgstore, where [pgstore.New] takes an injected pool and the
// appbuild recipe owns and closes it. Same shape, second backend.
//
// Nil: never returned nil by [Open] alongside a nil error. A nil *DB is
// a programming error, not a supported "no database" value.
type DB struct {
	db          *sql.DB
	opts        Options
	journalMode string
	lock        *processLock
}

// Open opens (creating if absent) the database at opts.Path and returns a
// handle that is ready to query.
//
// The caller OWNS the result and must [DB.Close] it. Handing it to a store or
// a config loader does NOT transfer ownership — both borrow it, so whoever
// opened the file is who closes it.
//
// Errors are surfaced unchanged rather than wrapped: the actionable ones are
// "another process holds the single-writer lock" and "WAL could not be
// enabled because this is a network or sync filesystem", and burying either
// under an "open store" prefix hides the part the operator needs.
func Open(ctx context.Context, opts Options) (*DB, error) {
	if opts.Path == "" {
		return nil, errors.New("sqlitedb: Options.Path is required")
	}
	if opts.BusyTimeout <= 0 {
		opts.BusyTimeout = defaultBusyTimeout
	}
	if opts.MaxOpenConns < 2 {
		opts.MaxOpenConns = defaultMaxOpenConns
	}

	// Take the single-writer lock BEFORE opening the database, so a second
	// process is refused before it can write anything.
	lock, err := acquireProcessLock(opts.Path)
	if err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", dsn(opts))
	if err != nil {
		_ = lock.release()
		return nil, fmt.Errorf("sqlitedb: open %s: %w", opts.Path, err)
	}
	db.SetMaxOpenConns(opts.MaxOpenConns)

	c := &DB{db: db, opts: opts, lock: lock}
	if err := c.init(ctx); err != nil {
		_ = c.Close()
		return nil, err
	}
	return c, nil
}

// init verifies the connection settings actually took and creates the schema.
func (c *DB) init(ctx context.Context) error {
	if err := c.db.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&c.journalMode); err != nil {
		return fmt.Errorf("sqlitedb: read journal_mode: %w", err)
	}
	if c.journalMode != "wal" && !c.opts.AllowNonWAL {
		return fmt.Errorf(
			"sqlitedb: WAL could not be enabled (journal_mode=%q) for %s — "+
				"this usually means the file is on a network or file-sync "+
				"filesystem (iCloud, Dropbox, SMB), where SQLite is not safe. "+
				"Move the project to local storage, or set AllowNonWAL if you "+
				"understand the risk",
			c.journalMode, c.opts.Path)
	}

	if err := c.verifyBusyTimeout(ctx); err != nil {
		return err
	}

	// Freshness must be decided BEFORE schemaSQL runs. schemaSQL is CREATE
	// TABLE IF NOT EXISTS throughout, so afterwards a brand-new database and
	// a pre-existing one are indistinguishable — sqlite_master reports the
	// tables either way. Asking first is the only way to know which this is.
	fresh, err := c.isFresh(ctx)
	if err != nil {
		return err
	}
	// Also before schemaSQL, and for a reason specific to this one column:
	// schemaSQL indexes relations(rel_record_id), which a pre-v4 database does
	// not have, so it must be added first or schemaSQL fails before the ladder
	// is ever reached. See ensureRelRecordIDColumn.
	if err := c.ensureRelRecordIDColumn(ctx); err != nil {
		return err
	}
	if _, err := c.db.ExecContext(ctx, schemaSQL); err != nil {
		return fmt.Errorf("sqlitedb: create schema: %w", err)
	}
	if err := c.migrate(ctx, fresh); err != nil {
		return err
	}
	if _, err := c.db.ExecContext(ctx, contentHashDDL); err != nil {
		return fmt.Errorf("sqlitedb: create content_hash triggers and indexes: %w", err)
	}
	// After the ladder rather than in schemaSQL: mentionsDDL indexes and
	// triggers on entities.mention_targets, which a pre-v17 table gains only
	// in the v17 rung. Every statement is IF NOT EXISTS, so this is a no-op on
	// a database that already has it.
	if _, err := c.db.ExecContext(ctx, mentionsDDL); err != nil {
		return fmt.Errorf("sqlitedb: create mention index: %w", err)
	}
	// After the ladder for the same reason: searchDDL reads
	// entities.search_text, which a pre-v22 table gains in the v22 rung.
	if _, err := c.db.ExecContext(ctx, searchDDL); err != nil {
		return fmt.Errorf("sqlitedb: create search index: %w", err)
	}
	// After the ladder for the same reason: channelTimersDDL indexes
	// channels.session_next_due_at, which a pre-v25 table gains in the v25
	// rung.
	if _, err := c.db.ExecContext(ctx, channelTimersDDL); err != nil {
		return fmt.Errorf("sqlitedb: create session due index: %w", err)
	}
	// After the ladder for the same reason: channelCreatorDDL indexes
	// channels.created_by_user, which a pre-v27 table gains in the v27
	// rung.
	if _, err := c.db.ExecContext(ctx, channelCreatorDDL); err != nil {
		return fmt.Errorf("sqlitedb: create channel creator index: %w", err)
	}
	return nil
}

// verifyBusyTimeout confirms the PRAGMA reached more than one connection.
//
// This exists because the failure it guards is SILENT: a per-connection PRAGMA
// applied to a single pooled connection reads back correctly while leaving
// every other connection at 0. Pinning two connections open simultaneously
// forces database/sql to open a genuinely different one, so a regression here
// (someone "simplifying" the DSN back to a db.Exec) fails at Open rather
// than as mysterious SQLITE_BUSY under load.
func (c *DB) verifyBusyTimeout(ctx context.Context) error {
	first, err := c.db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("sqlitedb: verify busy_timeout: %w", err)
	}
	defer first.Close()
	second, err := c.db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("sqlitedb: verify busy_timeout: %w", err)
	}
	defer second.Close()

	want := c.opts.BusyTimeout.Milliseconds()
	for i, conn := range []*sql.Conn{first, second} {
		var got int64
		if err := conn.QueryRowContext(ctx, "PRAGMA busy_timeout").Scan(&got); err != nil {
			return fmt.Errorf("sqlitedb: verify busy_timeout: %w", err)
		}
		if got != want {
			return fmt.Errorf(
				"sqlitedb: busy_timeout is %dms on connection %d, want %dms — "+
					"PRAGMAs must be set via DSN _pragma= parameters, not db.Exec",
				got, i, want)
		}
	}
	return nil
}

// JournalMode reports the journal mode actually in effect.
func (c *DB) JournalMode() string { return c.journalMode }

// DB exposes the pool, so config stored in this database can be read before a
// store exists and the two can share one connection.
//
// Returns a live handle, not a copy. Do not close it: close the [DB] that owns
// it, which is what tears down the pool and releases the single-writer lock.
func (c *DB) DB() *sql.DB { return c.db }

// Close tears down the pool and releases the single-writer lock.
//
// This is the ONLY thing that closes the database. A store or config loader
// built over this handle borrows it, so closing either of those leaves the
// file open — which is the point: they share it, and neither owns it.
func (c *DB) Close() error {
	err := c.db.Close()
	if lockErr := c.lock.release(); lockErr != nil && err == nil {
		err = lockErr
	}
	return err
}

// defaultBusyTimeout is how long a writer waits for the write lock before
// giving up. Generous on purpose — with the in-process write mutex below, a
// queued writer normally waits on the mutex rather than spending this budget,
// so reaching it means genuine contention worth waiting out.
const defaultBusyTimeout = 5 * time.Second

// defaultMaxOpenConns sizes the pool. Must be > 1: see the package doc on pool
// starvation.
const defaultMaxOpenConns = 8

// Options configures a store. The zero value is valid for every field except
// Path.
type Options struct {
	// Path is the database file. Required.
	Path string

	// BusyTimeout is how long a blocked writer waits. Zero uses
	// defaultBusyTimeout.
	BusyTimeout time.Duration

	// MaxOpenConns caps the connection pool. Values below 2 — the zero value
	// included — are raised to defaultMaxOpenConns, because a pool of one
	// deadlocks any Tx that runs concurrently with a read.
	MaxOpenConns int

	// AllowNonWAL permits opening a database where WAL could not be enabled.
	//
	// Default false, and that default is the point: WAL needs shared memory,
	// so it silently stays "delete" on most network and sync filesystems
	// (iCloud, Dropbox, SMB) — where SQLite is unsafe and the sidecar lock is
	// unreliable too. A desktop user who puts a project in iCloud otherwise
	// gets corruption with no diagnostic. Set this only for a deliberate,
	// understood exception.
	AllowNonWAL bool
}

// dsn builds the connection string. Every PRAGMA is a DSN parameter so it
// applies to EVERY pooled connection — see the package doc.
func dsn(opts Options) string {
	return fmt.Sprintf(
		"%s?_pragma=busy_timeout(%d)&_pragma=journal_mode(WAL)"+
			"&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(1)",
		opts.Path, opts.BusyTimeout.Milliseconds())
}

// schemaSQL is created unconditionally at Open, and is the shape of a
// FRESH database. It is CREATE TABLE IF NOT EXISTS throughout, so it is a
// silent no-op against an existing table of a different shape — carrying an
// older database forward is migrate.go's job, and every change here needs a
// matching step there.
var schemaSQL = `
CREATE TABLE IF NOT EXISTS entities (
	id          TEXT NOT NULL,
	-- face is the content-state coordinate (TKT-DOFYR1); '' is the DEFAULT
	-- state, so a faceless project stores exactly the rows it always did.
	--
	-- '' NOT NULL rather than NULL, matching pgstore: the face joins the
	-- primary key, and PK columns cannot be NULL. One convention everywhere —
	-- Go zero value, omitted frontmatter key, '' column.
	--
	-- The store only ever EQUALITY-MATCHES this value (see entity.Face), so
	-- one plain TEXT column suffices and keeps suffixing when multi-axis
	-- coordinates arrive: worlds compile to sets of concrete coordinates
	-- before they reach a store.
	face        TEXT NOT NULL DEFAULT '',
	type        TEXT NOT NULL,
	properties  TEXT NOT NULL DEFAULT '{}',
	content     TEXT NOT NULL DEFAULT '',
	updated_at  TEXT NOT NULL,
	-- last_edited_by_* record who made the most recent write, so the version
	-- sweep can attribute a create/update to its real author. NULL means the
	-- write carried no attribution. Added to older databases by the v7 rung.
	last_edited_by_user TEXT,
	last_edited_by_tool TEXT,
	-- origin_* record how the most recent write was produced (store.Origin),
	-- so the sweep can mark a copied entity's version as a copy. All NULL
	-- means a direct edit; see pgstore migration 0013. Added to older
	-- databases by the v9 rung.
	origin_kind        TEXT,
	origin_source      TEXT,
	origin_source_face TEXT,
	origin_source_type TEXT,
	origin_definition  TEXT,
	-- content_hash is the canonical hash the version sweep last computed for
	-- this row; NULL means not known. See contentHashDDL. Added to older
	-- databases by the v14 rung.
	content_hash       TEXT,
	-- mention_targets is the JSON array of entity ids the body mentions
	-- (TKT-Q6AKUN), written by the store with the body. NULL means not
	-- indexed yet; see mentionsDDL. Added to older databases by the v17 rung.
	mention_targets    TEXT,
	-- search_text is the lowered text the search index matches
	-- (search.IndexText), written by the store with the row. NULL means not
	-- indexed yet; see searchDDL. Added to older databases by the v22 rung.
	search_text        TEXT,
	PRIMARY KEY (id, face)
) STRICT;
` + entitiesTypeIDFaceIndexDDL + `
-- Entity IDs are case-insensitive IDENTITIES (BUG-3RCWNS): "abc" and "ABC"
-- cannot coexist. Enforced as a unique index on lower(id) rather than by
-- changing the column collation, exactly as pgstore does — the primary key
-- stays byte-exact so every id lookup keeps its semantics and index usage, and
-- casing is still PRESERVED on the row. Only the uniqueness rule widens.
--
-- The backends must agree on identity to stay substitutable: fsstore writes
-- "<id>.md" and so inherits the host filesystem's case folding, which would
-- silently drop one of the pair on macOS or Windows.
--
-- The face joins the key here too (pgstore's 0011 migration does the same):
-- states of ONE id legitimately share lower(id), so uniqueness is per
-- (lower(id), face). What the index does NOT catch — ('ABC','') alongside an
-- existing ('abc','draft') — is rejected by the write path's family probe
-- instead: a state requires ITS OWN default row, and 'abc' has none.
CREATE UNIQUE INDEX IF NOT EXISTS entities_id_lower_key ON entities(lower(id), face);

CREATE TABLE IF NOT EXISTS relations (
	from_id    TEXT NOT NULL,
	-- from_face is the state-specific TAIL (design doc §2.3). There is
	-- deliberately no to_face: heads stay entity-level, which is what makes
	-- cross-world dangling references impossible.
	from_face  TEXT NOT NULL DEFAULT '',
	rel_type   TEXT NOT NULL,
	to_id      TEXT NOT NULL,
	properties TEXT NOT NULL DEFAULT '{}',
	content    TEXT NOT NULL DEFAULT '',
	updated_at TEXT NOT NULL,
	-- rel_record_id is the stable surrogate that identifies a relation's
	-- version LINEAGE (TKT-4NU9ZD). It lives on the row rather than being
	-- reconstructed per sweep tick from the composite key, which is what
	-- dissolves the sweep-vs-synchronous-capture allocation race and the
	-- delete-recreate history-merge class of bugs: lineage is read straight
	-- off the row.
	--
	-- 0 means "not yet assigned". It is the value an existing row takes when
	-- the v3→v4 migration adds the column (SQLite requires a CONSTANT
	-- default), and backfillRelRecordIDs replaces it. On a fresh database
	-- nothing should ever keep it: CreateRelation mints an id from
	-- rel_record_seq.
	rel_record_id INTEGER NOT NULL DEFAULT 0,
	-- Same meaning as on entities.
	last_edited_by_user TEXT,
	last_edited_by_tool TEXT,
	content_hash        TEXT,
	PRIMARY KEY (from_id, from_face, rel_type, to_id)
) STRICT;
CREATE INDEX IF NOT EXISTS relations_from_idx ON relations(from_id);
CREATE INDEX IF NOT EXISTS relations_to_idx   ON relations(to_id);

CREATE TABLE IF NOT EXISTS attachments (
	entity_id  TEXT NOT NULL,
	property   TEXT NOT NULL,
	file_name  TEXT NOT NULL,
	data       BLOB NOT NULL,
	size       INTEGER NOT NULL,
	updated_at TEXT NOT NULL,
	PRIMARY KEY (entity_id, property, file_name)
) STRICT;
CREATE INDEX IF NOT EXISTS attachments_entity_idx ON attachments(entity_id);
` + projectFilesDDL + `
` + stateKVDDL + `
` + commentsDDL + `
` + channelsDDL + `
` + migrationStateDDL + `
` + versionSchemaSQL + `
` + versionTagsDDL + `
` + softDeleteDDL + `
` + searchKeyDDL

// softDeleteDDL holds soft-deleted entities and their hidden relations until
// the undo window closes (sqlitestore/softdelete.go). A marked row is MOVED
// here from entities/relations rather than flagged in place, so every read of
// the live tables stays correct without a predicate.
//
// The column lists repeat the live tables' columns in the same order, plus the
// mark itself; TestSoftDeleteTablesMatchLiveTables pins that, because a column
// added to entities but not here would be lost on restore.
//
// Shared between schemaSQL and the v9→v10 migration, like the other DDL
// constants, so a fresh database and a migrated one cannot differ.
const softDeleteDDL = `
CREATE TABLE IF NOT EXISTS marked_entities (
	id          TEXT NOT NULL,
	face        TEXT NOT NULL DEFAULT '',
	type        TEXT NOT NULL,
	properties  TEXT NOT NULL DEFAULT '{}',
	content     TEXT NOT NULL DEFAULT '',
	updated_at  TEXT NOT NULL,
	last_edited_by_user TEXT,
	last_edited_by_tool TEXT,
	origin_kind        TEXT,
	origin_source      TEXT,
	origin_source_face TEXT,
	origin_source_type TEXT,
	origin_definition  TEXT,
	content_hash       TEXT,
	mention_targets    TEXT,
	search_text        TEXT,
	deleted_at  TEXT NOT NULL,
	deleted_by  TEXT NOT NULL DEFAULT '',
	PRIMARY KEY (id, face)
) STRICT;
-- The id stays held while marked, case-folded like entities_id_lower_key.
CREATE INDEX IF NOT EXISTS marked_entities_id_lower_idx ON marked_entities(lower(id));

CREATE TABLE IF NOT EXISTS marked_relations (
	-- owner_id is the marked entity this edge comes back with. An edge between
	-- two marked entities belongs to one of them at a time.
	owner_id   TEXT NOT NULL,
	from_id    TEXT NOT NULL,
	from_face  TEXT NOT NULL DEFAULT '',
	rel_type   TEXT NOT NULL,
	to_id      TEXT NOT NULL,
	properties TEXT NOT NULL DEFAULT '{}',
	content    TEXT NOT NULL DEFAULT '',
	updated_at TEXT NOT NULL,
	rel_record_id INTEGER NOT NULL DEFAULT 0,
	last_edited_by_user TEXT,
	last_edited_by_tool TEXT,
	content_hash        TEXT,
	PRIMARY KEY (from_id, from_face, rel_type, to_id)
) STRICT;
CREATE INDEX IF NOT EXISTS marked_relations_owner_idx ON marked_relations(owner_id);`

// contentHashDDL keeps a live row's content_hash honest (BUG-1DWMYO,
// TASK-Y73Y9 in Atlas; pgstore migrations 0020 and 0021).
//
// The version sweep selects only rows whose stored hash is NULL, and writes
// back the hash it computes. A non-NULL hash therefore has to mean that the
// current lifecycle's latest version has that hash. Writers never set the
// column; these triggers clear it whenever that could stop being true,
// including on a write path that does not know about it:
//
//   - a hashed column changes;
//   - a version is inserted with another hash, or is a delete, which starts a
//     new lifecycle (the rename, delete and purge paths write versions outside
//     the sweep);
//   - a version is deleted (purge), which can change which version is latest;
//   - a row is inserted carrying a hash (soft-delete restore copies the whole
//     row back).
//
// CREATE ... IF NOT EXISTS leaves an existing trigger as it is, so changing
// a trigger body here needs a ladder rung that drops the old one.
//
// A spurious clear costs one more look by the sweep. The sweep's own write-back
// sets content_hash alone, so it does not fire the update triggers. The partial
// indexes serve the sweep's scan of unhashed rows.
//
// Not part of schemaSQL. SQLite resolves trigger columns when a trigger fires,
// not when it is created, so schemaSQL would install these on a v13 database
// before the rung adds the column. If the rung then failed, a v13 binary
// opening that database would fail every update with "no such column". init
// runs this after migrate instead, when the column exists either way.
const contentHashDDL = `
CREATE TRIGGER IF NOT EXISTS entities_clear_content_hash
AFTER UPDATE OF id, face, type, properties, content ON entities
WHEN NEW.content_hash IS NOT NULL
 AND (NEW.id IS NOT OLD.id OR NEW.face IS NOT OLD.face OR NEW.type IS NOT OLD.type
      OR NEW.properties IS NOT OLD.properties OR NEW.content IS NOT OLD.content)
BEGIN
	UPDATE entities SET content_hash = NULL WHERE id = NEW.id AND face = NEW.face;
END;
CREATE TRIGGER IF NOT EXISTS relations_clear_content_hash
AFTER UPDATE OF from_id, from_face, rel_type, to_id, properties, content ON relations
WHEN NEW.content_hash IS NOT NULL
 AND (NEW.from_id IS NOT OLD.from_id OR NEW.from_face IS NOT OLD.from_face
      OR NEW.rel_type IS NOT OLD.rel_type OR NEW.to_id IS NOT OLD.to_id
      OR NEW.properties IS NOT OLD.properties OR NEW.content IS NOT OLD.content)
BEGIN
	UPDATE relations SET content_hash = NULL
	 WHERE from_id = NEW.from_id AND from_face = NEW.from_face
	   AND rel_type = NEW.rel_type AND to_id = NEW.to_id;
END;
CREATE TRIGGER IF NOT EXISTS entities_insert_clear_content_hash
AFTER INSERT ON entities WHEN NEW.content_hash IS NOT NULL
BEGIN
	UPDATE entities SET content_hash = NULL WHERE id = NEW.id AND face = NEW.face;
END;
CREATE TRIGGER IF NOT EXISTS relations_insert_clear_content_hash
AFTER INSERT ON relations WHEN NEW.content_hash IS NOT NULL
BEGIN
	UPDATE relations SET content_hash = NULL WHERE rel_record_id = NEW.rel_record_id;
END;
CREATE TRIGGER IF NOT EXISTS entity_versions_insert_clear_content_hash
AFTER INSERT ON entity_versions
BEGIN
	UPDATE entities SET content_hash = NULL
	 WHERE id = NEW.entity_id AND face = NEW.face AND content_hash IS NOT NULL
	   AND (NEW.op = 'delete' OR content_hash IS NOT NEW.content_hash);
END;
CREATE TRIGGER IF NOT EXISTS entity_versions_delete_clear_content_hash
AFTER DELETE ON entity_versions
BEGIN
	UPDATE entities SET content_hash = NULL
	 WHERE id = OLD.entity_id AND face = OLD.face AND content_hash IS NOT NULL;
END;
CREATE TRIGGER IF NOT EXISTS relation_versions_insert_clear_content_hash
AFTER INSERT ON relation_versions
BEGIN
	UPDATE relations SET content_hash = NULL
	 WHERE rel_record_id = NEW.rel_record_id AND content_hash IS NOT NULL
	   AND (NEW.op = 'delete' OR content_hash IS NOT NEW.content_hash);
END;
CREATE TRIGGER IF NOT EXISTS relation_versions_delete_clear_content_hash
AFTER DELETE ON relation_versions
BEGIN
	UPDATE relations SET content_hash = NULL
	 WHERE rel_record_id = OLD.rel_record_id AND content_hash IS NOT NULL;
END;
CREATE INDEX IF NOT EXISTS entities_unhashed_idx ON entities(updated_at) WHERE content_hash IS NULL;
CREATE INDEX IF NOT EXISTS relations_unhashed_idx ON relations(updated_at) WHERE content_hash IS NULL;`

// entitiesTypeIDFaceIndexDDL serves a type page in every face selection
// (TKT-KQXVF7). Each list shape orders by (id, face), and a world picks one
// row per id, so (type, id, face) lets a page walk the index in order: the
// implicit-face page filters face inside it, and the all-faces, explicit-face
// and world pages read it as is. It replaced entities_type_idx (type), which
// made every type page sort the whole type.
//
// Shared between schemaSQL (fresh databases) and the v10→v11 migration
// (existing ones), for the same reason projectFilesDDL is.
const entitiesTypeIDFaceIndexDDL = `
CREATE INDEX IF NOT EXISTS entities_type_id_face_idx ON entities(type, id, face);`

// projectFilesDDL carries the operator-authored config — schema.yaml,
// data-entry.yaml, acl.yaml, scripts/, templates/, custom/ — so a single
// database file can be a complete, shippable rela project rather than the data
// half of one.
//
// Flat path keys with no directory rows: listing is a prefix scan, which is
// all any consumer needs, and it keeps the two config backends agreeing about
// what a directory is (a filesystem has real ones; this has keys containing
// slashes). BLOB rather than TEXT because custom/ and apps/ carry fonts and
// images alongside the YAML.
//
// Shared between schemaSQL (fresh databases) and the v1→v2 migration
// (existing ones). One definition, not two copies: when duplicated DDL drifts,
// a fresh database and a migrated one end up with different shapes — precisely
// the failure the version-stamping apparatus exists to prevent, arriving by
// the one route it cannot detect.
const projectFilesDDL = `
CREATE TABLE IF NOT EXISTS project_files (
	path       TEXT PRIMARY KEY,
	content    BLOB NOT NULL,
	updated_at TEXT NOT NULL
) STRICT;`

// stateKVDDL carries rela's runtime state: the document render cache, user
// settings, the operator's logo and theme, the CalDAV alias table.
//
// Keys are opaque strings — validation is state.ValidatedKV's job at the
// wiring site, so both backends accept exactly the same keys and neither can
// drift. BLOB because a logo is bytes, not text.
//
// Shared between schemaSQL (fresh databases) and the v2→v3 migration
// (existing ones), for the same reason projectFilesDDL is: two copies drift,
// and a fresh database ending up with a different shape from a migrated one
// is the failure the version stamp exists to prevent.
const stateKVDDL = `
CREATE TABLE IF NOT EXISTS state_kv (
	key        TEXT PRIMARY KEY,
	value      BLOB NOT NULL,
	updated_at TEXT NOT NULL
) STRICT;`

// commentsDDL carries entity commentary (TKT-OGTVJW), backing
// internal/comments/sqlitecomments.
//
// In the database rather than in .rela/comments/*.yaml for the reason
// versioning is (TKT-4NU9ZD) and state_kv is not (TKT-L1A3PH): a comment is
// content ABOUT content, so it must travel with the rows it annotates. An
// operator copying or shipping rela.db as "the project" would otherwise find
// every entity present and every remark on them left behind.
//
// Deliberately NO foreign key to entities. A comment is a remark about an
// entity, not a fact in the operator's domain model, and the feature lives
// outside store.Store, entitymanager, the audit log and /_schema by design. An
// FK would hand this table's lifecycle to the store's cascade machinery, when
// the comment SERVICE owns it (Service.EntityDeleted, Store.Rename). The cost
// is that a comment can outlive its entity — the same property the file
// backend has, and survivable, since every read is by target key so an
// unreachable row is invisible.
//
// target_key is entity.FormatStateRef(id, face): the bare id for the default
// face, "id@face" otherwise — the same key filecomments uses for its filename,
// so all four backends agree on what identifies a thread.
//
// Note SQLite's LIKE is ASCII case-INSENSITIVE by default while "=" is
// byte-exact, so the two arms of `target_key = ? OR target_key LIKE ?` would
// match different row sets. sqlitecomments handles that in its queries (a
// byte-exact substr guard beside the LIKE) rather than here: COLLATE on the
// column does NOT affect LIKE, and `PRAGMA case_sensitive_like` is global, so
// setting it would silently change sqlitestore's queries — which rely on the
// folding deliberately (see sqlitestore/rename.go).
//
// anchor is JSON text because comments.Anchor is a discriminated union whose
// text kind carries a six-field descriptor set; columns would mean six mostly-
// NULL ones plus a migration per new kind, and the type's doc requires that
// adding a kind not migrate stored comments. Nothing queries inside it.
//
// Shared between schemaSQL (fresh databases) and the v4→v5 migration
// (existing ones), for the same reason the two DDL blocks above are.
// migrationStateDDL carries the data-migration record (TKT-XCJ0Y2): which
// migrations have run against this database and the schema shape its content
// conforms to.
//
// In the database rather than in a file beside it, for versioning's reason
// (TKT-4NU9ZD) rather than state_kv's: this describes the CONTENT, so shipping
// rela.db without it would hand over every entity alongside a record claiming
// no migration had ever run — and the next run would replay all of them.
//
// A single row, pinned by the id CHECK. There is exactly one store per
// database file, so a key would be a column with one possible value.
const migrationStateDDL = `
CREATE TABLE IF NOT EXISTS migration_state (
	id    INTEGER PRIMARY KEY CHECK (id = 1),
	state TEXT NOT NULL
) STRICT;`

const commentsDDL = `
CREATE TABLE IF NOT EXISTS comments (
	id          TEXT NOT NULL,
	target_key  TEXT NOT NULL,
	target_type TEXT NOT NULL,
	author      TEXT NOT NULL,
	created_at  TEXT NOT NULL,
	updated_at  TEXT,
	anchor      TEXT NOT NULL,
	body        TEXT NOT NULL,
	resolved    INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY (target_key, id)
) STRICT;
-- Serves List: one target's thread in contract order (oldest first, id
-- breaking ties), as an index range scan rather than a sort.
CREATE INDEX IF NOT EXISTS comments_thread_idx ON comments(target_key, created_at, id);`

// channelsDDL holds the realtime channels of internal/channels/sqlchannels;
// that package doc describes the tables. Like comments they live in the
// database so a copied rela.db carries them, and there is no foreign key to
// entities: the channel service owns their rename and delete lifecycle.
// Shared between schemaSQL and the v17 to v18 rung.
const channelsDDL = `
CREATE TABLE IF NOT EXISTS channels (
	id          INTEGER PRIMARY KEY,
	anchor_id   TEXT NOT NULL,
	anchor_face TEXT NOT NULL DEFAULT '',
	type        TEXT NOT NULL,
	name        TEXT NOT NULL DEFAULT '',
	tomb        TEXT NOT NULL DEFAULT '',
	incarnation TEXT NOT NULL DEFAULT '',
	state       TEXT NOT NULL,
	title       TEXT NOT NULL DEFAULT '',
	created_by  TEXT NOT NULL,
	created_at  INTEGER NOT NULL,
	closed_at   INTEGER NOT NULL DEFAULT 0,
	archived_at INTEGER NOT NULL DEFAULT 0,
	reopened_at INTEGER NOT NULL DEFAULT 0,
	last_seq    INTEGER NOT NULL DEFAULT 0,
	rev         INTEGER NOT NULL DEFAULT 0,
	session_state      TEXT NOT NULL DEFAULT '',
	session_gen        INTEGER NOT NULL DEFAULT 0,
	session_entered_at INTEGER NOT NULL DEFAULT 0,
	session_next_due_at INTEGER NOT NULL DEFAULT 0,
	session_restarts    TEXT NOT NULL DEFAULT '',
	session_reminders   TEXT NOT NULL DEFAULT '',
	created_by_user     TEXT NOT NULL DEFAULT '',
	UNIQUE (anchor_id, anchor_face, type, name, tomb)
) STRICT;
CREATE INDEX IF NOT EXISTS channels_state_idx ON channels(tomb, state);

CREATE TABLE IF NOT EXISTS channel_messages (
	channel_id  INTEGER NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
	seq         INTEGER NOT NULL,
	rev         INTEGER NOT NULL,
	reply_to    INTEGER NOT NULL DEFAULT 0,
	reply_count INTEGER NOT NULL DEFAULT 0,
	pending_at  INTEGER NOT NULL DEFAULT 0,
	doc         TEXT NOT NULL,
	PRIMARY KEY (channel_id, seq)
) STRICT;
CREATE INDEX IF NOT EXISTS channel_messages_rev_idx ON channel_messages(channel_id, rev);
CREATE INDEX IF NOT EXISTS channel_messages_pending_idx ON channel_messages(pending_at) WHERE pending_at > 0;

CREATE TABLE IF NOT EXISTS channel_audit (
	id        INTEGER PRIMARY KEY,
	anchor_id TEXT NOT NULL,
	tomb      TEXT NOT NULL DEFAULT '',
	record    TEXT NOT NULL
) STRICT;
CREATE INDEX IF NOT EXISTS channel_audit_anchor_idx ON channel_audit(anchor_id, tomb, id);
-- A cardinality of '' is a type recorded before v21, when only the anchor
-- mode was kept. The v20 to v21 rung adds the column with the same default,
-- so a fresh and an upgraded table have the same shape.
CREATE TABLE IF NOT EXISTS channel_types (
	type        TEXT PRIMARY KEY,
	anchor_mode TEXT NOT NULL,
	cardinality TEXT NOT NULL DEFAULT ''
) STRICT;` + channelTypeRefsDDL + channelRetentionDDL

// channelTypeRefsDDL holds the ref properties recorded for each channel
// type (see channels.TypeRecord). Shared between channelsDDL and the v19 to
// v20 rung.
const channelTypeRefsDDL = `
CREATE TABLE IF NOT EXISTS channel_type_refs (
	type TEXT NOT NULL,
	ref  TEXT NOT NULL,
	PRIMARY KEY (type, ref)
) STRICT;`

// channelTimersDDL is the index the realtime timer loop lists due sessions
// through (channels.Store.ListSessionsDue): one range per channel type, with
// the lifecycle state in the key so closed channels are skipped by the scan.
// It is partial, so the index holds only sessions with a due time. pgstore
// migration 0032 has the same shape. It runs after the ladder, as
// mentionsDDL does, because it names a column the v25 rung adds.
const channelTimersDDL = `
CREATE INDEX IF NOT EXISTS channels_session_due_idx ON channels(type, state, session_next_due_at)
	WHERE session_next_due_at > 0 AND tomb = '';`

// channelCreatorDDL is the index the inbox lists the channels a user
// created through (channels.Store.ListWaiting): one range per user,
// channel type and channel state. It is partial, so rows without a creator
// user are left out. The state column makes it the best match for the
// creator statement, whose type and state filter channels_type_state_idx
// would serve as well. pgstore migration 0034 has the same shape. It runs after the ladder, as
// channelTimersDDL does, because it names a column the v27 rung adds.
const channelCreatorDDL = `
CREATE INDEX IF NOT EXISTS channels_creator_idx ON channels(created_by_user, type, state)
	WHERE tomb = '' AND created_by_user <> '';`

// searchKeyDDL is the key table of the search index; see searchDDL. It is in
// schemaSQL because it depends on no column a rung adds.
const searchKeyDDL = `
CREATE TABLE IF NOT EXISTS entity_search_key (
	key  INTEGER PRIMARY KEY,
	id   TEXT NOT NULL,
	face TEXT NOT NULL,
	UNIQUE (id, face)
) STRICT;`

// channelRetentionDDL holds what each tombstoned batch of channels recorded
// when it was filed (channels.Filing), and the indexes retention reads
// through: due_at for the purge poll, and (type, state, tomb) for the close
// and archive sweeps and the type-record drop. pgstore migration 0030 has
// the same shape. Shared between channelsDDL and the v22 to v23 rung.
const channelRetentionDDL = `
CREATE TABLE IF NOT EXISTS channel_tombstones (
	anchor_id   TEXT NOT NULL,
	tomb        TEXT NOT NULL,
	anchor_face TEXT NOT NULL DEFAULT '',
	policy      TEXT NOT NULL DEFAULT '',
	purge_after INTEGER NOT NULL DEFAULT 0,
	type_after  TEXT NOT NULL DEFAULT '',
	due_at      INTEGER NOT NULL DEFAULT 0,
	pending     INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY (anchor_id, tomb)
) STRICT;
CREATE INDEX IF NOT EXISTS channel_tombstones_due_idx ON channel_tombstones(pending, due_at);
CREATE INDEX IF NOT EXISTS channels_type_state_idx ON channels(type, state, tomb);`

// searchDDL is the full-text search index (DEC-10Z731): an FTS5 table with
// the trigram tokenizer, which answers substring queries as pgstore's LIKE
// over its trigram index does.
//
// The indexed text is entities.search_text, which the store writes in Go with
// every row (search.IndexText, the same text as pgstore's search_text): the
// id, the string-valued top-level properties and the body, lowered by Go. The
// tokenizer is case_sensitive, and the needle is lowered by Go too, so the
// index matches exactly when search.MatchTextFields finds a field
// (BUG-YG126C). A tokenizer that folds case itself folds differently from Go
// (U+0528 and U+0130, for example), so a row could match through a hidden
// field while the Go judgement credited a visible one. A NULL search_text is
// a row the store has not indexed yet; it is not in the index until the
// store's reindex pass fills it in.
//
// Triggers keep the index in step inside the writing transaction. That makes
// it as current as the table on every write path (store writes, renames, soft
// delete, purge, bulk import) with no observer and no rebuild after a crash.
//
// An index row is not keyed by the entities rowid (RR-44YDTU). entities is
// keyed by (id, face) and has no INTEGER PRIMARY KEY, so its rowid is not
// stable: VACUUM may renumber it, and the index would then point at other
// rows. entity_search_key gives each (id, face) a key that VACUUM keeps,
// because an INTEGER PRIMARY KEY column is the rowid and VACUUM preserves it.
// Each index row's rowid is that key, and search joins through the key table
// to entities by (id, face). Every trigger finds its key through the UNIQUE
// (id, face) index, so a write costs O(log n), never a scan of the index.
//
// A rename updates the key row in place, so the entity keeps its key. The
// inserts use INSERT OR IGNORE on the key and INSERT OR REPLACE on the index,
// so neither fails on a row it finds already there: a key freed by a delete
// can be reused, and an INSERT OR REPLACE on entities removes the old row
// without firing the delete trigger.
//
// It runs after the ladder, as mentionsDDL does, because it reads
// entities.search_text, which a pre-v22 table gains only in the v22 rung.
//
//nolint:gosec // G202: joins constant DDL only; no input reaches it
var searchDDL = searchKeyDDL + `
CREATE VIRTUAL TABLE IF NOT EXISTS entity_search USING fts5(body, tokenize = 'trigram case_sensitive 1');
CREATE INDEX IF NOT EXISTS entities_search_pending_idx ON entities(id, face) WHERE search_text IS NULL;
CREATE TRIGGER IF NOT EXISTS entity_search_insert AFTER INSERT ON entities BEGIN
	INSERT OR IGNORE INTO entity_search_key(id, face) VALUES (NEW.id, NEW.face);
	INSERT OR REPLACE INTO entity_search(rowid, body) VALUES (` + searchKeyOf("NEW") + `, coalesce(NEW.search_text, ''));
END;
CREATE TRIGGER IF NOT EXISTS entity_search_update AFTER UPDATE OF id, face, search_text ON entities BEGIN
	UPDATE entity_search_key SET id = NEW.id, face = NEW.face WHERE id = OLD.id AND face = OLD.face;
	INSERT OR IGNORE INTO entity_search_key(id, face) VALUES (NEW.id, NEW.face);
	INSERT OR REPLACE INTO entity_search(rowid, body) VALUES (` + searchKeyOf("NEW") + `, coalesce(NEW.search_text, ''));
END;
CREATE TRIGGER IF NOT EXISTS entity_search_delete AFTER DELETE ON entities BEGIN
	DELETE FROM entity_search WHERE rowid = ` + searchKeyOf("OLD") + `;
	DELETE FROM entity_search_key WHERE id = OLD.id AND face = OLD.face;
END;`

// searchKeyOf is the SQL for the index key of the entities row named row.
// The DDL constants call it at package init, so its argument is a literal.
func searchKeyOf(row string) string {
	return `(SELECT key FROM entity_search_key WHERE id = ` + row + `.id AND face = ` + row + `.face)`
}

// mentionsDDL is the backlink index (TKT-Q6AKUN). The store writes each row's
// targets into entities.mention_targets as a JSON array, computed in Go with
// the body; the triggers fan that array out into entity_mention_targets
// inside the writing transaction, as the search triggers do for entity_search.
//
// An index row names its source by (id, face), not by the entities rowid,
// which VACUUM may renumber (see searchDDL). A rename or a new face moves the
// entries with the row. Every trigger first drops the entries of the row it
// writes: an INSERT OR REPLACE on entities removes the old row without firing
// the delete trigger, so its entries would otherwise outlive it.
var mentionsDDL = `
CREATE TABLE IF NOT EXISTS entity_mention_targets (
	target      TEXT NOT NULL,
	source_id   TEXT NOT NULL,
	source_face TEXT NOT NULL,
	PRIMARY KEY (target, source_id, source_face)
) STRICT, WITHOUT ROWID;
CREATE INDEX IF NOT EXISTS entity_mention_targets_source_idx ON entity_mention_targets(source_id, source_face);
-- The store's reindex pass reads this to find rows written before v17.
CREATE INDEX IF NOT EXISTS entities_mentions_pending_idx ON entities(id, face) WHERE mention_targets IS NULL;
CREATE TRIGGER IF NOT EXISTS entity_mentions_insert AFTER INSERT ON entities BEGIN
	DELETE FROM entity_mention_targets WHERE source_id = NEW.id AND source_face = NEW.face;
	INSERT OR IGNORE INTO entity_mention_targets(target, source_id, source_face)
		SELECT value, NEW.id, NEW.face FROM json_each(NEW.mention_targets);
END;
CREATE TRIGGER IF NOT EXISTS entity_mentions_update AFTER UPDATE OF id, face, mention_targets ON entities BEGIN
	DELETE FROM entity_mention_targets WHERE source_id = OLD.id AND source_face = OLD.face;
	DELETE FROM entity_mention_targets WHERE source_id = NEW.id AND source_face = NEW.face;
	INSERT OR IGNORE INTO entity_mention_targets(target, source_id, source_face)
		SELECT value, NEW.id, NEW.face FROM json_each(NEW.mention_targets);
END;
CREATE TRIGGER IF NOT EXISTS entity_mentions_delete AFTER DELETE ON entities BEGIN
	DELETE FROM entity_mention_targets WHERE source_id = OLD.id AND source_face = OLD.face;
END;`

// dropSearchSQL removes the search index and its triggers, keeping the key
// table. The v13 rung ran it to drop the rowid-keyed v12 index, and the v22
// rung to drop the case-folding index; searchDDL then creates the current one
// after the ladder. The triggers must go too: they share the new triggers'
// names, so CREATE TRIGGER IF NOT EXISTS would otherwise keep the old ones.
const dropSearchSQL = `DROP TRIGGER IF EXISTS entity_search_insert;
DROP TRIGGER IF EXISTS entity_search_update;
DROP TRIGGER IF EXISTS entity_search_delete;
DROP TABLE IF EXISTS entity_search;`
