package pgstore

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ConnectDirect opens one connection, outside any pool, to the database dsn
// names.
//
// The DSN is the operator's pool DSN, so it may carry pgxpool-only keys such
// as pool_max_conns. pgx.Connect would send those to the server as runtime
// parameters, and the server refuses the connection (BUG-JQO2PH,
// BUG-A8GG1CD). Parsing with pgxpool.ParseConfig removes them.
// TestNoDirectPgxConnect (internal/archguard) keeps pgx.Connect out of
// non-test code so every direct connection goes through here.
func ConnectDirect(ctx context.Context, dsn string) (*pgx.Conn, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		// pgx redacts the password in parse errors, so this is safe to wrap.
		return nil, fmt.Errorf("parse database DSN: %w", err)
	}
	return pgx.ConnectConfig(ctx, cfg.ConnConfig)
}

// PinSearchPath returns base with `search_path` set to schema and `public`,
// and every other setting of base kept.
//
// `public` stays on the path because the search backend's `pg_trgm`
// operators live there.
//
// The result is parsed again and refused unless it resolves to exactly that
// path. PostgreSQL reads setting names without regard to case, and pgx sends
// runtime parameters in no fixed order, so a second spelling of search_path
// in base, or one inside options, is refused too: it could win on some
// connections.
func PinSearchPath(base, schema string) (string, error) {
	searchPath := schema + ",public"
	out, err := withSetting(base, "search_path", searchPath)
	if err != nil {
		return "", err
	}
	cfg, err := pgx.ParseConfig(out)
	if err != nil {
		return "", fmt.Errorf("pin search_path: %w", err)
	}
	if got := cfg.RuntimeParams["search_path"]; got != searchPath {
		return "", fmt.Errorf("pin search_path: the DSN resolves to search_path %q, want %q", got, searchPath)
	}
	for k, v := range cfg.RuntimeParams {
		if (k != "search_path" && strings.EqualFold(k, "search_path")) ||
			(strings.EqualFold(k, "options") && strings.Contains(strings.ToLower(v), "search_path")) {

			return "", fmt.Errorf("pin search_path: the base DSN sets search_path through %q; remove it", k)
		}
	}
	return out, nil
}

// WithDatabase returns base pointed at database, with every other setting
// kept.
func WithDatabase(base, database string) (string, error) {
	out, err := withSetting(base, "dbname", database)
	if err != nil {
		return "", err
	}
	cfg, err := pgx.ParseConfig(out)
	if err != nil {
		return "", fmt.Errorf("set dbname: %w", err)
	}
	if cfg.Database != database {
		return "", fmt.Errorf("set dbname: the DSN resolves to database %q, want %q", cfg.Database, database)
	}
	return out, nil
}

// withSetting returns base with one keyword set, written in the dialect base
// uses: a URL query parameter, or a trailing keyword for the key/value form
// ("host=x dbname=y"), where a later keyword overrides an earlier one.
//
// Rebuilding the DSN from parsed fields instead would drop every setting not
// copied across, such as `sslmode=verify-full`, its root certificate and
// pool_max_conns, and silently weaken TLS (BUG-BR9CXF). Callers parse the
// result again to check the setting took.
func withSetting(base, key, value string) (string, error) {
	if _, err := pgx.ParseConfig(base); err != nil {
		// pgx redacts the password in parse errors, so this is safe to wrap.
		return "", fmt.Errorf("parse DSN: %w", err)
	}
	if strings.HasPrefix(base, "postgres://") || strings.HasPrefix(base, "postgresql://") {
		u, err := url.Parse(base)
		if err != nil {
			// Not wrapped: a url.Parse error quotes the whole URL,
			// password included.
			return "", errors.New("parse DSN: not a valid URL")
		}
		q := u.Query()
		q.Set(key, value)
		u.RawQuery = q.Encode()
		return u.String(), nil
	}
	quoted := strings.ReplaceAll(strings.ReplaceAll(value, `\`, `\\`), `'`, `\'`)
	return strings.TrimSpace(base) + " " + key + "='" + quoted + "'", nil
}
