package sqlitestore

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/storeutil"
)

// AttachFile stores a file attachment on an entity, keyed by
// (entity_id, property, file_name). The entity must exist
// (store.ErrNotFound otherwise). The reader is fully consumed into memory
// and persisted as a BLOB, matching memstore/pgstore: SQLite has
// incremental BLOB I/O, but only for a blob whose size is fixed at insert
// time, which we do not know before reading the stream.
//
// The file name is stored verbatim after store.ValidateFileName. It is NOT
// normalized or collision-suffixed here — store.NormalizeFileName and
// store.SuffixOnCollision are write-path policy (internal/attachment),
// applied before the name reaches any backend. A store that silently
// rewrote the name would return a different key than the caller asked for.
func (s *Store) AttachFile(ctx context.Context, entityID, property, fileName string, r io.Reader) error {
	if err := storeutil.ValidateProperty(property); err != nil {
		return err
	}
	if err := store.ValidateFileName(fileName); err != nil {
		return err
	}

	// Read to completion BEFORE touching the database. The backstop cap is
	// the one every backend enforces so no storage path is unbounded (the
	// API layer caps at its own ingress). Doing this first is what makes a
	// failed replace non-destructive: an oversize or erroring stream returns
	// before any row is written, so an existing same-named attachment keeps
	// its old bytes rather than being truncated mid-write.
	data, err := io.ReadAll(storeutil.LimitAttachmentReader(r))
	if err != nil {
		if errors.Is(err, store.ErrAttachmentTooLarge) {
			return err
		}
		return fmt.Errorf("sqlitestore: read attachment %q: %w", fileName, err)
	}

	// Existence is checked in the same statement as the insert: an
	// INSERT..SELECT whose source is the entities row writes zero rows when
	// the entity is absent, so the check cannot race a concurrent delete the
	// way a separate SELECT-then-INSERT could.
	const q = `
		INSERT INTO attachments (entity_id, property, file_name, data, size, updated_at)
		SELECT id, ?, ?, ?, ?, ? FROM entities WHERE id = ?
		ON CONFLICT (entity_id, property, file_name)
		DO UPDATE SET data = excluded.data,
		              size = excluded.size,
		              updated_at = excluded.updated_at`
	res, err := s.q().ExecContext(ctx, q,
		property, fileName, data, int64(len(data)), time.Now().UTC().Format(timeFmt), entityID)
	if err != nil {
		return fmt.Errorf("sqlitestore: attach %q to %s: %w", fileName, entityID, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("sqlitestore: attach %q to %s: %w", fileName, entityID, err)
	}
	if n == 0 {
		// The SELECT matched no entity row.
		return store.ErrNotFound
	}
	return nil
}

// ReadAttachment returns a reader over the stored bytes, or
// store.ErrNotFound. The bytes are already in memory, so the returned
// closer is a no-op — but callers still own it and must Close, since other
// backends (fsstore) hand back a real file handle.
func (s *Store) ReadAttachment(ctx context.Context, entityID, property, fileName string) (io.ReadCloser, error) {
	const q = `SELECT data FROM attachments WHERE entity_id = ? AND property = ? AND file_name = ?`
	var data []byte
	err := s.q().QueryRowContext(ctx, q, entityID, property, fileName).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, store.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("sqlitestore: read attachment %q on %s: %w", fileName, entityID, err)
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

// DeleteAttachment removes one attachment, leaving its siblings on the same
// property untouched. Returns store.ErrNotFound if absent.
func (s *Store) DeleteAttachment(ctx context.Context, entityID, property, fileName string) error {
	const q = `DELETE FROM attachments WHERE entity_id = ? AND property = ? AND file_name = ?`
	res, err := s.q().ExecContext(ctx, q, entityID, property, fileName)
	if err != nil {
		return fmt.Errorf("sqlitestore: delete attachment %q on %s: %w", fileName, entityID, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("sqlitestore: delete attachment %q on %s: %w", fileName, entityID, err)
	}
	if n == 0 {
		return store.ErrNotFound
	}
	return nil
}

// ListAttachments lists an entity's attachments. Returns store.ErrNotFound
// if the entity does not exist — distinct from an existing entity with no
// attachments, which yields an empty slice.
func (s *Store) ListAttachments(ctx context.Context, entityID string) ([]store.AttachmentInfo, error) {
	var exists int
	err := s.q().QueryRowContext(ctx, `SELECT 1 FROM entities WHERE id = ?`, entityID).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, store.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("sqlitestore: list attachments for %s: %w", entityID, err)
	}

	// size is read from the stored column rather than length(data) so the
	// query never has to load the blobs just to report their sizes.
	//
	// ContentType is deliberately left zero: it is never written (no column
	// holds it), because the service layer derives content type from the file
	// name. This matches pgstore, where the column exists but stays ''.
	const q = `SELECT property, file_name, size FROM attachments
	           WHERE entity_id = ? ORDER BY property ASC, file_name ASC`
	rows, err := s.q().QueryContext(ctx, q, entityID)
	if err != nil {
		return nil, fmt.Errorf("sqlitestore: list attachments for %s: %w", entityID, err)
	}
	defer rows.Close()

	var result []store.AttachmentInfo
	for rows.Next() {
		info := store.AttachmentInfo{EntityID: entityID}
		if err := rows.Scan(&info.Property, &info.FileName, &info.Size); err != nil {
			return nil, fmt.Errorf("sqlitestore: scan attachment for %s: %w", entityID, err)
		}
		result = append(result, info)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlitestore: list attachments for %s: %w", entityID, err)
	}
	return result, nil
}
