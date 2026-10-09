package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/output"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/tokenstore"
)

// TokenCmd manages the OAuth tokens connector scripts use (TKT-01KZSO).
// The values are never printed: `set` reads them from stdin only, so they
// stay out of shell history and `ps`, and `status` reports state, not
// content.
type TokenCmd struct {
	Set    TokenSetCmd    `cmd:"" help:"Store a connection's refresh token, read from stdin."`
	Status TokenStatusCmd `cmd:"" help:"Show whether each connection has a usable token (never the token)."`
	Delete TokenDeleteCmd `cmd:"" help:"Delete a connection's stored token."`
}

// tokenAdmin is what the token commands call. tokenstore.Broker satisfies
// it; every method takes the connection's refresh lock.
type tokenAdmin interface {
	Set(ctx context.Context, name string, t tokenstore.Token) error
	Delete(ctx context.Context, name string) error
	Status(ctx context.Context, name string) (tokenstore.Status, error)
	Connections() tokenstore.Connections
}

// tokenStdin is where `rela token set` reads; a variable so tests can feed it.
var tokenStdin io.Reader = os.Stdin

// maxTokenInput bounds what `rela token set` reads. tokenstore.ParseInput
// refuses more than 16 KiB; the extra room lets it say so.
const maxTokenInput = 64 << 10

// TokenSetCmd stores a token. Input is a bare refresh token or the
// provider's JSON token response.
type TokenSetCmd struct {
	Name string `arg:"" help:"Connection name, as declared in connections.yaml."`
}

// Run reads the token from stdin and stores it under the connection's lock.
func (c *TokenSetCmd) Run(ctx context.Context, svc *writeServices) error {
	admin, err := svc.tokenAdmin()
	if err != nil {
		return err
	}
	if f, ok := tokenStdin.(*os.File); ok && isTerminal(f) {
		return errors.New("pipe the token in, for example: ./consent.py | rela token set " + c.Name +
			"; a token typed at a prompt would be echoed")
	}
	data, err := io.ReadAll(io.LimitReader(tokenStdin, maxTokenInput))
	if err != nil {
		return fmt.Errorf("read stdin: %w", err)
	}
	tok, err := tokenstore.ParseInput(data, time.Now())
	if err != nil {
		return err
	}
	if err := admin.Set(ctx, c.Name, tok); err != nil {
		return fmt.Errorf("store token for %s: %w", c.Name, err)
	}
	recordToken(ctx, svc.Audit, audit.OpTokenSet, c.Name)
	out.WriteSuccess("Stored the token for %s.", c.Name)
	return nil
}

// TokenDeleteCmd removes a stored token.
type TokenDeleteCmd struct {
	Name string `arg:"" help:"Connection name."`
}

// Run deletes the token under the connection's lock. Deleting a token that
// is not stored succeeds.
func (c *TokenDeleteCmd) Run(ctx context.Context, svc *writeServices) error {
	admin, err := svc.tokenAdmin()
	if err != nil {
		return err
	}
	if err := admin.Delete(ctx, c.Name); err != nil {
		return fmt.Errorf("delete token for %s: %w", c.Name, err)
	}
	recordToken(ctx, svc.Audit, audit.OpTokenDelete, c.Name)
	out.WriteSuccess("Deleted the token for %s.", c.Name)
	return nil
}

// TokenStatusCmd reports token state per connection.
type TokenStatusCmd struct {
	Names []string `arg:"" optional:"" help:"Connection names (default: every declared connection)."`
}

// tokenStatusJSON is one row of `rela token status -o json`.
type tokenStatusJSON struct {
	Name         string `json:"name"`
	Declared     bool   `json:"declared"`
	State        string `json:"state"`
	ExpiresAt    string `json:"access_expires_at,omitempty"`
	NeedsConsent string `json:"needs_consent_since,omitempty"`
	Error        string `json:"error,omitempty"`
}

// Run prints one row per connection.
func (c *TokenStatusCmd) Run(ctx context.Context, svc *writeServices) error {
	admin, err := svc.tokenAdmin()
	if err != nil {
		return err
	}
	names := c.Names
	if len(names) == 0 {
		for n := range admin.Connections() {
			names = append(names, string(n))
		}
		sort.Strings(names)
	}
	if len(names) == 0 {
		out.WriteMessage("No connections are declared in %s.", tokenstore.ConnectionsFile)
		return nil
	}
	rows := make([]tokenStatusJSON, 0, len(names))
	for _, n := range names {
		st, err := admin.Status(ctx, n)
		if err != nil {
			return err
		}
		rows = append(rows, tokenStatusRow(st))
	}
	if out.Format == output.FormatJSON {
		enc := json.NewEncoder(out.Out)
		enc.SetIndent("", "  ")
		return enc.Encode(rows)
	}
	for _, r := range rows {
		line := r.Name + ": " + r.State
		switch {
		case r.Error != "":
			line += " (" + r.Error + ")"
		case r.NeedsConsent != "":
			line += " since " + r.NeedsConsent + "; run the consent flow and `rela token set " + r.Name + "`"
		case r.ExpiresAt != "":
			line += ", access token expires " + r.ExpiresAt
		}
		if !r.Declared {
			line += " [not in " + tokenstore.ConnectionsFile + "]"
		}
		out.WriteMessage("%s", line)
	}
	return nil
}

func tokenStatusRow(st tokenstore.Status) tokenStatusJSON {
	r := tokenStatusJSON{Name: string(st.Name), Declared: st.Declared}
	switch {
	case st.Err != nil:
		r.State = "unreadable"
		r.Error = tokenReadReason(st.Err)
	case !st.Stored:
		r.State = "missing"
	case !st.NeedsConsentAt.IsZero():
		r.State = "needs_consent"
		r.NeedsConsent = st.NeedsConsentAt.UTC().Format(time.RFC3339)
	case st.AccessFresh:
		r.State = "ok"
		r.ExpiresAt = st.ExpiresAt.UTC().Format(time.RFC3339)
	default:
		// The access token is stale or absent; the next use refreshes it.
		r.State = "ok"
	}
	return r
}

// tokenReadReason says why a stored token does not open, in terms an
// operator can act on.
func tokenReadReason(err error) string {
	switch {
	case errors.Is(err, tokenstore.ErrWrongKey):
		return "sealed with a different token_key; restore that key or set the token again"
	case errors.Is(err, tokenstore.ErrCorrupt):
		return "the stored record is damaged; set the token again"
	default:
		return err.Error()
	}
}

// recordToken audits a token change by connection name only, never the
// value.
func recordToken(ctx context.Context, sink audit.Audit, op, name string) {
	if sink == nil {
		return
	}
	sink.Record(audit.Record{
		Time:      time.Now().UTC(),
		Op:        op,
		Principal: principal.From(ctx),
		Summary:   "connection=" + name,
	})
}

// isTerminal reports whether f is a character device, which is what an
// interactive stdin is.
func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}
