-- pgstore schema, version 34: the creator's user of each channel (TKT-ZZ4PNX).
--
-- created_by_user holds the user of the channel's created_by author
-- (channels.Author.User), written when the channel is created and never
-- changed after. The inbox lists the channels a user created through it
-- (channels.Store.ListWaiting): created_by is a JSON text column, and
-- reading it at query time has no index and fails on values jsonb rejects.
--
-- Rows written before this migration are filled from created_by. A row
-- whose created_by is not valid JSON, or holds the escape \u0000 that jsonb
-- refuses, keeps '': its creator gets no creator entry, and nobody else
-- does either. The helper function catches the cast error per row, which
-- works on every supported version (pg_input_is_valid needs PG16), and is
-- dropped again. strpos, not LIKE: LIKE reads the backslash as its escape
-- character. Only rows still at '' are written, so a re-run changes nothing.
--
-- channels_creator_idx serves the creator branch: one range per user,
-- channel type and channel state, partial so rows without a creator user
-- are left out.
-- sqlitedb's channelCreatorDDL has the same shape.
--
-- NEW FILE rather than an amendment to 0033: Migrate tracks only the
-- version number, so a deployment already past 33 would never see an edit.

ALTER TABLE channels ADD COLUMN IF NOT EXISTS created_by_user TEXT NOT NULL DEFAULT '';

CREATE FUNCTION rela_creator_user(t text) RETURNS text LANGUAGE plpgsql AS $$
BEGIN
    IF strpos(t, '\u0000') > 0 THEN
        RETURN '';
    END IF;
    BEGIN
        RETURN COALESCE(t::jsonb ->> 'user', '');
    EXCEPTION WHEN others THEN
        RETURN '';
    END;
END;
$$;

UPDATE channels SET created_by_user = rela_creator_user(created_by) WHERE created_by_user = '';

DROP FUNCTION rela_creator_user(text);

CREATE INDEX IF NOT EXISTS channels_creator_idx ON channels (created_by_user, type, state)
    WHERE tomb = '' AND created_by_user <> '';
