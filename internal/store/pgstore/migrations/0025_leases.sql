-- pgstore schema, version 25: advisory entity leases (TKT-6QC5EL).
--
-- Backs internal/lease/pglease. A lease is a short, renewable claim by one
-- holder on one entity. The single-process tiers keep leases in one state.KV
-- document; several rela-server processes against one database need one row
-- per lease, so that exactly one claim wins across processes.
--
-- Rows live in the tenant's schema like every other table here, so a
-- schema-per-tenant deployment scopes leases for free.
--
-- # Not graph content
--
-- No foreign key to entities. The lease service owns its rename and delete
-- lifecycle through the entitymanager alias hook, and a lease on a deleted
-- entity is harmless until then: readers resolve entities through their
-- visibility gate first.
--
-- Ids and hashes are matched byte-exactly, hence COLLATE "C" as on
-- entities.id.

CREATE TABLE leases (
    -- The entity id: the whole face family, never one face.
    entity_id   TEXT        COLLATE "C" PRIMARY KEY,
    -- Hash of the lease id handed to the claimant. The plaintext id is never
    -- stored.
    lease_hash  TEXT        COLLATE "C" NOT NULL,
    -- The key renew, release and the per-identity cap match on.
    identity    TEXT        COLLATE "C" NOT NULL,
    holder_user TEXT        NOT NULL,
    holder_tool TEXT        NOT NULL,
    label       TEXT        NOT NULL,
    token       BIGINT      NOT NULL,
    since       TIMESTAMPTZ NOT NULL,
    expires     TIMESTAMPTZ NOT NULL,
    status      TEXT        NOT NULL
);

-- Renew and release find a lease by its hash; a hash names one lease.
CREATE UNIQUE INDEX leases_hash_idx ON leases (lease_hash);

-- Serves ListByHolder and the per-identity cap.
CREATE INDEX leases_identity_idx ON leases (identity);

-- Serves the prune of expired leases on every claim.
CREATE INDEX leases_expires_idx ON leases (expires);

-- One row: the last token issued and the version watchers poll. A row rather
-- than a sequence, because both must move inside the writing transaction: a
-- sequence advances before commit, so a watcher could see the new version,
-- read the old rows, and then miss the change.
CREATE TABLE lease_counters (
    singleton BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (singleton),
    token     BIGINT  NOT NULL,
    version   BIGINT  NOT NULL
);

INSERT INTO lease_counters (singleton, token, version) VALUES (TRUE, 0, 0);
