-- pgstore schema, version 20: version tags (TKT-VO6VG9).
--
-- A version tag names one entity version row ("the version we last sent to
-- the board", a sync connector's merge base). It lives beside the versions
-- rather than inside the entity: a version number recorded in the entity
-- would change the entity and mint a newer version, so it would always be
-- one behind.
--
-- A tag holds the version's vseq, not its ordinal. vseq comes from
-- version_seq and never renumbers; ordinals are assigned at read time and
-- shift when purge removes a row. Which tags a face has is decided at read
-- time by the lineage walk and the lifecycle fence (see store.VersionTagger),
-- so a rename needs no tag rewrite.
--
-- The foreign key is a backstop for purge: a version row a tag points at
-- cannot be deleted while the tag exists. Purge refuses such a row unless
-- forced, and a forced purge deletes the tag rows first, in the same
-- transaction. The key needs a unique index on entity_versions(vseq). The
-- primary key is (entity_id, face, vseq), which does not make vseq unique on
-- its own, although version_seq already does in practice.
--
-- LOCK NOTE: the unique index is a NON-CONCURRENT build inside Migrate's one
-- transaction, so it briefly blocks writes to entity_versions on a large
-- table. Ordinary entity writes do not touch entity_versions; the sweep and
-- the synchronous rename/delete capture wait for it.
CREATE UNIQUE INDEX entity_versions_vseq_key ON entity_versions (vseq);

-- face repeats the tagged row's face. It is redundant with vseq and kept so
-- a tag row reads on its own in a diagnostic query. name is unique per
-- version; uniqueness per lineage and face is kept by the writer, which
-- deletes same-name rows across the lineage before inserting, under the
-- version lock.
CREATE TABLE version_tags (
    vseq           BIGINT      NOT NULL REFERENCES entity_versions (vseq) ON DELETE RESTRICT,
    face           TEXT        COLLATE "C" NOT NULL DEFAULT '',
    name           TEXT        COLLATE "C" NOT NULL,
    tagged_by_user TEXT        NOT NULL DEFAULT '',
    tagged_by_tool TEXT        NOT NULL DEFAULT '',
    tagged_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (vseq, name)
);
