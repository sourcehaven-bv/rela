-- pgstore schema, version 27: the ref properties of each channel type's
-- record (TKT-ZU7P13).
--
-- A property recorded as a ref holds entity ids in stored messages. The
-- channel service refuses a realtime.yaml that declares it as another type,
-- because the ids would then render as plain text to readers who may not see
-- the entity. Rows are only ever added.
CREATE TABLE channel_type_refs (
    type TEXT COLLATE "C" NOT NULL,
    ref  TEXT COLLATE "C" NOT NULL,
    PRIMARY KEY (type, ref)
);
