-- pgstore schema, version 45: records follow their owner (TKT-J7Q9C4K).
--
-- A record row belongs to an owner entity, the whole face family. When the
-- family leaves the database for good its rows go with it; when it is
-- renamed its rows move to the new id. Ids can be reused, so a row left
-- behind would be served to the readers of a later entity with the same id.
--
-- Triggers do this rather than a hook after the commit. They run inside the
-- delete or rename statement's own transaction: if they fail, the entity
-- write rolls back, and no write path can skip them. A post-commit hook can
-- fail after the delete has committed, which leaves the rows behind.
--
-- The owner is gone when no live face and no soft-deleted face of the id is
-- left. A soft delete moves the faces to marked_entities first, so the rows
-- stay for a restore; the purge that deletes the marked faces drops them.
-- The AFTER ROW triggers of one statement fire when the statement has
-- finished, so a delete of every face sees none left.
--
-- A ledger group claim is released with the last row of its group, as
-- sqlrecords' DropOwner does. Only the dropped owner's groups are checked.
--
-- Every statement may run again on a schema that already has it.

CREATE OR REPLACE FUNCTION rela_records_drop_owner() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF EXISTS (SELECT 1 FROM entities WHERE id = OLD.id)
       OR EXISTS (SELECT 1 FROM marked_entities WHERE id = OLD.id) THEN
        RETURN NULL;
    END IF;
    DELETE FROM record_groups g
     WHERE (g.type, g.grp) IN (SELECT r.type, r.grp FROM records r WHERE r.owner = OLD.id AND r.grp <> '')
       AND NOT EXISTS (SELECT 1 FROM records o
                        WHERE o.type = g.type AND o.grp = g.grp AND o.owner <> OLD.id);
    DELETE FROM records WHERE owner = OLD.id;
    RETURN NULL;
END
$$;

CREATE OR REPLACE FUNCTION rela_records_rekey_owner() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    UPDATE records SET owner = NEW.id WHERE owner = OLD.id;
    RETURN NULL;
END
$$;

DROP TRIGGER IF EXISTS entities_records_drop_owner ON entities;
CREATE TRIGGER entities_records_drop_owner
    AFTER DELETE ON entities
    FOR EACH ROW EXECUTE FUNCTION rela_records_drop_owner();

DROP TRIGGER IF EXISTS marked_entities_records_drop_owner ON marked_entities;
CREATE TRIGGER marked_entities_records_drop_owner
    AFTER DELETE ON marked_entities
    FOR EACH ROW EXECUTE FUNCTION rela_records_drop_owner();

DROP TRIGGER IF EXISTS entities_records_rekey_owner ON entities;
CREATE TRIGGER entities_records_rekey_owner
    AFTER UPDATE OF id ON entities
    FOR EACH ROW
    WHEN (OLD.id IS DISTINCT FROM NEW.id)
    EXECUTE FUNCTION rela_records_rekey_owner();

-- Rows a failed post-commit hook, which these triggers replace, left behind.
DELETE FROM records
 WHERE owner NOT IN (SELECT id FROM entities UNION SELECT id FROM marked_entities);
DELETE FROM record_groups g
 WHERE NOT EXISTS (SELECT 1 FROM records r WHERE r.type = g.type AND r.grp = g.grp);
