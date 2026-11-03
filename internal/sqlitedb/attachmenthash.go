package sqlitedb

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
)

// addAttachmentSHA256 is the v28 to v29 rung (TKT-1TS55R): the sha256
// column of attachments, and the hash of every row that has none yet.
//
// The rowids are read first and each blob is then read, hashed and written
// back one row at a time, so memory holds one attachment, not the table, and
// no UPDATE runs while a cursor walks the table. All of it commits with the
// version bump in the one migration transaction. Only rows whose hash is
// still empty are written, so a re-run changes nothing.
func addAttachmentSHA256(ctx context.Context, conn *sql.Conn) error {
	var n int
	if err := conn.QueryRowContext(ctx,
		`SELECT count(*) FROM pragma_table_info('attachments') WHERE name = 'sha256'`,
	).Scan(&n); err != nil {
		return fmt.Errorf("probe attachments.sha256: %w", err)
	}
	if n == 0 {
		if _, err := conn.ExecContext(ctx,
			`ALTER TABLE attachments ADD COLUMN sha256 TEXT NOT NULL DEFAULT ''`); err != nil {
			return fmt.Errorf("add attachments.sha256: %w", err)
		}
	}
	rowids, err := attachmentRowidsWithoutHash(ctx, conn)
	if err != nil {
		return err
	}
	for _, rowid := range rowids {
		var data []byte
		if err := conn.QueryRowContext(ctx,
			`SELECT data FROM attachments WHERE rowid = ?`, rowid).Scan(&data); err != nil {
			return fmt.Errorf("read attachment rowid %d: %w", rowid, err)
		}
		sum := sha256.Sum256(data)
		if _, err := conn.ExecContext(ctx,
			`UPDATE attachments SET sha256 = ? WHERE rowid = ?`, hex.EncodeToString(sum[:]), rowid); err != nil {
			return fmt.Errorf("hash attachment rowid %d: %w", rowid, err)
		}
	}
	return nil
}

// attachmentRowidsWithoutHash collects the rows whose hash is still empty.
func attachmentRowidsWithoutHash(ctx context.Context, conn *sql.Conn) ([]int64, error) {
	rows, err := conn.QueryContext(ctx, `SELECT rowid FROM attachments WHERE sha256 = '' ORDER BY rowid`)
	if err != nil {
		return nil, fmt.Errorf("scan attachments for hash backfill: %w", err)
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan attachment rowid: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("scan attachments for hash backfill: %w", err)
	}
	return ids, nil
}
