package pgstore

import (
	"bytes"
	"context"
	"errors"
	"io"

	"github.com/jackc/pgx/v5"

	"github.com/Sourcehaven-BV/rela/internal/store"
)

// AttachFile stores (or replaces) a file attachment on an entity. The entity
// must exist (store.ErrNotFound otherwise). The reader is fully consumed into
// memory and persisted as BYTEA, matching memstore's behavior. A single
// (entity_id, property) holds one attachment; re-attaching overwrites it.
func (s *Store) AttachFile(ctx context.Context, entityID, property, fileName string, r io.Reader) error {
	if err := validateProperty(property); err != nil {
		return err
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback after commit is a no-op

	var exists bool
	if err := tx.QueryRow(ctx, `SELECT true FROM entities WHERE id = $1`, entityID).Scan(&exists); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return store.ErrNotFound
		}
		return err
	}

	const q = `
		INSERT INTO attachments (entity_id, property, file_name, bytes, updated_at)
		VALUES ($1, $2, $3, $4, now())
		ON CONFLICT (entity_id, property)
		DO UPDATE SET file_name = excluded.file_name, bytes = excluded.bytes,
		             updated_at = now(), seq = nextval('rela_seq')`
	if _, err := tx.Exec(ctx, q, entityID, property, fileName, data); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ReadAttachment returns a reader over the stored bytes, or store.ErrNotFound.
func (s *Store) ReadAttachment(ctx context.Context, entityID, property string) (io.ReadCloser, error) {
	const q = `SELECT bytes FROM attachments WHERE entity_id = $1 AND property = $2`
	var data []byte
	err := s.db.QueryRow(ctx, q, entityID, property).Scan(&data)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, store.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

// DeleteAttachment removes an attachment. Returns store.ErrNotFound if absent.
func (s *Store) DeleteAttachment(ctx context.Context, entityID, property string) error {
	const q = `DELETE FROM attachments WHERE entity_id = $1 AND property = $2`
	tag, err := s.db.Exec(ctx, q, entityID, property)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return store.ErrNotFound
	}
	return nil
}

// ListAttachments lists an entity's attachments. Returns store.ErrNotFound if
// the entity does not exist.
func (s *Store) ListAttachments(ctx context.Context, entityID string) ([]store.AttachmentInfo, error) {
	var exists bool
	err := s.db.QueryRow(ctx, `SELECT true FROM entities WHERE id = $1`, entityID).Scan(&exists)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, store.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	const q = `SELECT entity_id, property, file_name, content_type, octet_length(bytes)
	           FROM attachments WHERE entity_id = $1 ORDER BY property ASC`
	rows, err := s.db.Query(ctx, q, entityID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []store.AttachmentInfo
	for rows.Next() {
		var info store.AttachmentInfo
		if err := rows.Scan(&info.EntityID, &info.Property, &info.FileName, &info.ContentType, &info.Size); err != nil {
			return nil, err
		}
		result = append(result, info)
	}
	return result, rows.Err()
}
