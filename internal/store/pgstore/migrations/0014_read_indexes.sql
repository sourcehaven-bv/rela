-- pgstore schema, version 14: read-path indexes and a search ranking that
-- scales (TKT-1U8XYN).
--
-- entities_type_id_idx: the default page of a type ("this type, by id,
-- rows N..M") is `WHERE face = '' AND type = $1 ORDER BY id LIMIT k`. The
-- primary key (id, face) cannot serve that with a type filter; (type)
-- alone leaves a sort over the type. A partial (type, id) index on the
-- default face makes the page an index range scan. Kanban boards fetch
-- exactly this shape, 100 rows at a time.
--
-- entities_id_prefix_idx: HighestID (sequential id allocation on every
-- create) runs `WHERE id LIKE 'PFX-%' AND face = ''`. The unique
-- (lower(id), face) index cannot serve a prefix pattern; text_pattern_ops
-- can, turning a per-create scan of the table into a range scan of one
-- prefix.
--
-- entities_search_tsv_idx is dropped: nothing has ever queried it. Search
-- is a trigram LIKE over search_text (entities_search_trgm_idx) ranked by
-- similarity — see search.go — so the tsvector index was pure write
-- amplification on every entity write (TKT-ZZL53L).
--
-- search_text is rebuilt with id, then string properties, then content,
-- so that the first kilobyte is the identity and the title rather than the
-- opening of the body. The store composes it that way from now on
-- (entitySearchText); this brings existing rows in line. Ranking uses that
-- prefix: similarity over a whole 5 KB body per candidate row cost seconds
-- on a common word, and title similarity is what a reader means anyway.
-- lower() here vs strings.ToLower in Go can differ on exotic Unicode; the
-- next write of a row recomposes it in Go, and the mismatch could only
-- ever affect matching on such characters within a body.

CREATE INDEX IF NOT EXISTS entities_type_id_idx ON entities (type, id) WHERE face = '';

CREATE INDEX IF NOT EXISTS entities_id_prefix_idx ON entities (id text_pattern_ops) WHERE face = '';

DROP INDEX IF EXISTS entities_search_tsv_idx;

UPDATE entities SET search_text =
    lower(id) || E'\n' ||
    COALESCE((SELECT string_agg(lower(p.value), E'\n' ORDER BY p.key)
              FROM jsonb_each_text(properties) p
              WHERE jsonb_typeof(properties -> p.key) = 'string'), '') || E'\n' ||
    lower(content);
