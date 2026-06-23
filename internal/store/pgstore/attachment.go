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
// memory and persisted as BYTEA, matching memstore's behavior, up to the shared
// store.MaxAttachmentBytes backstop (the same cap every backend enforces so no
// backend is ever unbounded; the API layer caps at its own ingress). A single
// (entity_id, property) holds one attachment; re-attaching overwrites it.
func (s *Store) AttachFile(ctx context.Context, entityID, property, fileName string, r io.Reader) error {
	if err := validateProperty(property); err != nil {
		return err
	}
	if err := store.ValidateFileName(fileName); err != nil {
		return err
	}
	// Read at most the cap +1 so we can detect an over-limit upload without
	// buffering the entire (potentially huge) reader.
	data, err := io.ReadAll(io.LimitReader(r, store.MaxAttachmentBytes+1))
	if err != nil {
		return err
	}
	if int64(len(data)) > store.MaxAttachmentBytes {
		return store.ErrAttachmentTooLarge
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

	// Append: keyed by (entity_id, property, file_name). A same-name
	// re-attach replaces only that one row; sibling files are untouched.
	// content_type is intentionally left at its '' default: content type is
	// derived from the file name at the service layer (attachment.Service.List
	// via contentTypeForName), so the column is never written. ListAttachments
	// selects it for forward-compatibility but callers should not rely on it.
	const q = `
		INSERT INTO attachments (entity_id, property, file_name, bytes, updated_at)
		VALUES ($1, $2, $3, $4, now())
		ON CONFLICT (entity_id, property, file_name)
		DO UPDATE SET bytes = excluded.bytes,
		             updated_at = now(), seq = nextval('rela_seq')`
	if _, err := tx.Exec(ctx, q, entityID, property, fileName, data); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ReadAttachment returns a reader over the stored bytes, or store.ErrNotFound.
func (s *Store) ReadAttachment(ctx context.Context, entityID, property, fileName string) (io.ReadCloser, error) {
	const q = `SELECT bytes FROM attachments WHERE entity_id = $1 AND property = $2 AND file_name = $3`
	var data []byte
	err := s.db.QueryRow(ctx, q, entityID, property, fileName).Scan(&data)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, store.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

// DeleteAttachment removes an attachment. Returns store.ErrNotFound if absent.
func (s *Store) DeleteAttachment(ctx context.Context, entityID, property, fileName string) error {
	const q = `DELETE FROM attachments WHERE entity_id = $1 AND property = $2 AND file_name = $3`
	tag, err := s.db.Exec(ctx, q, entityID, property, fileName)
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

	// content_type is always '' here — it is never written (see AttachFile); the
	// service layer derives content type from the file name. Selected for
	// forward-compatibility only.
	const q = `SELECT entity_id, property, file_name, content_type, octet_length(bytes)
	           FROM attachments WHERE entity_id = $1 ORDER BY property ASC, file_name ASC`
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
