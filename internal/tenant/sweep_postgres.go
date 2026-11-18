//go:build postgres

package tenant

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// sweepTablesQuery lists the active tenants whose schema exists, with which
// of the due-work columns it has. A tenant last opened before a migration
// lacks a table or a column, such as channels without session_next_due_at,
// so that part is left out of the due query rather than failing it.
const sweepTablesQuery = `SELECT t.org_id, t.schema_name,
	COALESCE(bool_or(c.relname = 'flow_runs' AND a.attname = 'next_due_at'), false),
	COALESCE(bool_or(c.relname = 'channels' AND a.attname = 'session_next_due_at'), false),
	COALESCE(bool_or(c.relname = 'river_job' AND a.attname = 'scheduled_at'), false)
	FROM ` + controlTable + ` t
	JOIN pg_namespace n ON n.nspname = t.schema_name
	LEFT JOIN pg_class c ON c.relnamespace = n.oid AND c.relkind IN ('r', 'p')
		AND c.relname IN ('flow_runs', 'channels', 'river_job')
	LEFT JOIN pg_attribute a ON a.attrelid = c.oid AND NOT a.attisdropped
		AND a.attname IN ('next_due_at', 'session_next_due_at', 'scheduled_at')
	WHERE t.status = 'active'
	GROUP BY t.org_id, t.schema_name
	ORDER BY t.org_id`

// sweepTimeout bounds one sweep pass's reads, and sweepTenantTimeout one
// tenant's, so one slow tenant cannot use up the pass.
const (
	sweepTimeout       = 30 * time.Second
	sweepTenantTimeout = 5 * time.Second
)

// dueQuery builds the statement that reads schema's earliest due work, in
// microseconds since the epoch, as the flow and channel stores keep it. Each
// part reads a partial index's first row. It returns "" when the schema has
// none of the tables.
func dueQuery(schema string, flows, channels, jobs bool) string {
	var parts []string
	if flows {
		parts = append(parts, `(SELECT min(next_due_at) FROM `+
			pgx.Identifier{schema, "flow_runs"}.Sanitize()+` WHERE next_due_at > 0)`)
	}
	if channels {
		parts = append(parts, `(SELECT min(session_next_due_at) FROM `+
			pgx.Identifier{schema, "channels"}.Sanitize()+
			` WHERE session_next_due_at > 0 AND tomb = '' AND state = 'open')`)
	}
	if jobs {
		parts = append(parts, `(SELECT (extract(epoch FROM min(scheduled_at)) * 1000000)::bigint FROM `+
			pgx.Identifier{schema, "river_job"}.Sanitize()+
			` WHERE state IN ('available', 'scheduled', 'retryable'))`)
	}
	switch len(parts) {
	case 0:
		return ""
	case 1:
		return "SELECT " + parts[0]
	}
	// LEAST ignores NULL arguments.
	return "SELECT LEAST(" + strings.Join(parts, ", ") + ")"
}

// dueTenants implements [dueReader] for the postgres control reader. A
// tenant whose due work cannot be read is logged and skipped, so it does
// not stop the others being swept.
func (c pgControl) dueTenants(ctx context.Context, until time.Time) ([]DueTenant, error) {
	ctx, cancel := context.WithTimeout(ctx, sweepTimeout)
	defer cancel()
	type candidate struct {
		org, query string
	}
	rows, err := c.pool.Query(ctx, sweepTablesQuery)
	if err != nil {
		return nil, fmt.Errorf("tenant: list tenants to sweep: %w", err)
	}
	var cands []candidate
	for rows.Next() {
		var org, schema string
		var flows, channels, jobs bool
		if err := rows.Scan(&org, &schema, &flows, &channels, &jobs); err != nil {
			rows.Close()
			return nil, fmt.Errorf("tenant: list tenants to sweep: %w", err)
		}
		if q := dueQuery(schema, flows, channels, jobs); q != "" {
			cands = append(cands, candidate{org: org, query: q})
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("tenant: list tenants to sweep: %w", err)
	}

	limit := until.UnixMicro()
	var due []DueTenant
	for i, cand := range cands {
		if ctx.Err() != nil {
			slog.Warn("tenant: sweep pass ran out of time; the rest waits for the next pass",
				"tenants_left", len(cands)-i)
			break
		}
		at, err := readDue(ctx, c.pool, cand.query)
		if err != nil {
			slog.Warn("tenant: sweep skips a tenant whose due work cannot be read", "org_id", cand.org, "error", err)
			continue
		}
		if at != nil && *at <= limit {
			due = append(due, DueTenant{OrgID: cand.org, DueAt: time.UnixMicro(*at).UTC()})
		}
	}
	return due, nil
}

// readDue runs one tenant's due query within [sweepTenantTimeout].
func readDue(ctx context.Context, q rowQuerier, query string) (*int64, error) {
	ctx, cancel := context.WithTimeout(ctx, sweepTenantTimeout)
	defer cancel()
	var at *int64
	if err := q.QueryRow(ctx, query).Scan(&at); err != nil {
		return nil, err
	}
	return at, nil
}
