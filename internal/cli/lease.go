package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/lease"
	"github.com/Sourcehaven-BV/rela/internal/output"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// LeaseCmd groups the advisory lease verbs. A lease tells people and other
// agents that someone is working on an entity; it does not block writes.
//
// The lease id is the secret that renews and releases a lease. Only claim
// prints it, and no command logs it. renew and release also read it from
// stdin (--lease-id-stdin), so it need not appear on a command line.
type LeaseCmd struct {
	Claim   LeaseClaimCmd   `cmd:"" help:"Claim an advisory lease on an entity; prints the lease id."`
	Renew   LeaseRenewCmd   `cmd:"" help:"Extend your lease before it expires, and optionally change its status."`
	Release LeaseReleaseCmd `cmd:"" help:"Release your lease. Succeeds whether or not the lease still existed."`
	List    LeaseListCmd    `cmd:"" help:"List the live leases you hold, oldest first."`
}

// leaseService is the lease capability the lease commands use. The wiring
// site passes the project's *lease.Service, which applies the read and
// update checks itself.
type leaseService interface {
	Claim(ctx context.Context, req lease.ClaimRequest) (lease.Claimed, error)
	Renew(ctx context.Context, leaseID string, ttl time.Duration, status *string) (lease.Lease, error)
	Release(ctx context.Context, leaseID string) error
	Mine(ctx context.Context) ([]lease.Lease, error)
}

// leaseServices is the binding the lease commands take.
//
// Nil Svc: accepted; the project was assembled without a lease backend, and
// every command answers [lease.ErrNotEnabled].
type leaseServices struct {
	Svc leaseService
}

const (
	// leaseListHint is the list command the lease limit text cites.
	leaseListHint = "`rela lease list`"
	// maxLeaseIDInput bounds the line --lease-id-stdin reads.
	maxLeaseIDInput = 1024
)

var (
	// errLeaseTTL answers a --ttl outside the range the service accepts.
	errLeaseTTL = fmt.Errorf("--ttl: a duration from %s to %s (such as 15m), or whole seconds from %d to %d",
		lease.MinTTL, lease.MaxTTL, int(lease.MinTTL/time.Second), int(lease.MaxTTL/time.Second))
)

// leaseTextError shows a lease error as its [lease.UserText] and keeps the
// error in its chain for errors.Is.
type leaseTextError struct {
	text string
	err  error
}

func (e *leaseTextError) Error() string { return e.text }
func (e *leaseTextError) Unwrap() error { return e.err }

// leaseStdin is where --lease-id-stdin reads; a variable so tests can feed it.
var leaseStdin io.Reader = os.Stdin

func (l *leaseServices) svc() (leaseService, error) {
	if l == nil || l.Svc == nil {
		return nil, &leaseTextError{text: lease.NotEnabledText, err: lease.ErrNotEnabled}
	}
	return l.Svc, nil
}

// LeaseClaimCmd claims a lease.
type LeaseClaimCmd struct {
	ID     string `arg:"" help:"Entity ID, or ID@face for an entity with content states. The lease covers every face."`
	TTL    string `help:"Lease duration: 30s to 1h as a duration (15m) or whole seconds (900). Default 15m."`
	Label  string `help:"Name that tells apart agents running as the same user (up to 32 letters, digits, '_', '.' or '-')."`
	Status string `help:"Short note on what you are doing (at most 140 characters). Everyone who can read the entity sees it."`
}

// claimJSON is the `rela lease claim -o json` answer.
type claimJSON struct {
	LeaseID   string `json:"lease_id"`
	Token     uint64 `json:"token"`
	ExpiresAt string `json:"expires_at"`
}

// Run claims the lease and prints the lease id, token and expiry.
func (c *LeaseClaimCmd) Run(ctx context.Context, l *leaseServices) error {
	svc, err := l.svc()
	if err != nil {
		return err
	}
	ttl, err := parseLeaseTTL(c.TTL)
	if err != nil {
		return err
	}
	id := strings.TrimSpace(c.ID)
	claimed, err := svc.Claim(ctx, lease.ClaimRequest{Addr: id, TTL: ttl, Label: c.Label, Status: c.Status})
	if err != nil {
		return leaseClaimFailed(ctx, id, err)
	}
	res := claimJSON{
		LeaseID: claimed.LeaseID, Token: claimed.Lease.Token, ExpiresAt: leaseTime(claimed.Lease.Expires),
	}
	if out.Format == output.FormatJSON {
		return writeLeaseJSON(res)
	}
	out.WriteMessage("lease_id: %s\ntoken: %d\nexpires_at: %s", res.LeaseID, res.Token, res.ExpiresAt)
	return nil
}

// LeaseRenewCmd renews a lease. Status is a pointer so an absent flag keeps
// the status and `--status ""` clears it.
type LeaseRenewCmd struct {
	LeaseID      string  `arg:"" optional:"" name:"lease-id" help:"The lease id claim printed."`
	LeaseIDStdin bool    `name:"lease-id-stdin" help:"Read the lease id from stdin instead."`
	TTL          string  `help:"New duration from now: 30s to 1h as a duration (15m) or whole seconds (900). Default 15m."`
	Status       *string `help:"New status (at most 140 characters). Omit to keep the status; an empty string clears it."`
}

// renewJSON is the `rela lease renew -o json` answer.
type renewJSON struct {
	Token     uint64 `json:"token"`
	ExpiresAt string `json:"expires_at"`
}

// Run renews the lease and prints its token and new expiry.
func (c *LeaseRenewCmd) Run(ctx context.Context, l *leaseServices) error {
	svc, err := l.svc()
	if err != nil {
		return err
	}
	leaseID, err := readLeaseID(c.LeaseID, c.LeaseIDStdin)
	if err != nil {
		return err
	}
	ttl, err := parseLeaseTTL(c.TTL)
	if err != nil {
		return err
	}
	got, err := svc.Renew(ctx, leaseID, ttl, c.Status)
	if err != nil {
		return leaseFailed(ctx, "renew", err)
	}
	res := renewJSON{Token: got.Token, ExpiresAt: leaseTime(got.Expires)}
	if out.Format == output.FormatJSON {
		return writeLeaseJSON(res)
	}
	out.WriteMessage("token: %d\nexpires_at: %s", res.Token, res.ExpiresAt)
	return nil
}

// LeaseReleaseCmd releases a lease.
type LeaseReleaseCmd struct {
	LeaseID      string `arg:"" optional:"" name:"lease-id" help:"The lease id claim printed."`
	LeaseIDStdin bool   `name:"lease-id-stdin" help:"Read the lease id from stdin instead."`
}

// Run releases the lease. The answer is the same whether or not it existed.
func (c *LeaseReleaseCmd) Run(ctx context.Context, l *leaseServices) error {
	svc, err := l.svc()
	if err != nil {
		return err
	}
	leaseID, err := readLeaseID(c.LeaseID, c.LeaseIDStdin)
	if err != nil {
		return err
	}
	if err := svc.Release(ctx, leaseID); err != nil {
		return leaseFailed(ctx, "release", err)
	}
	if out.Format == output.FormatJSON {
		return writeLeaseJSON(map[string]bool{"released": true})
	}
	out.WriteMessage("released")
	return nil
}

// LeaseListCmd lists the caller's own leases.
type LeaseListCmd struct{}

// ownLeaseJSON is one `rela lease list -o json` row. It has no lease id.
type ownLeaseJSON struct {
	ID        string `json:"id"`
	Since     string `json:"since"`
	ExpiresAt string `json:"expires_at"`
	Status    string `json:"status,omitempty"`
	Label     string `json:"label,omitempty"`
}

// Run prints the live leases of the caller, oldest first.
func (c *LeaseListCmd) Run(ctx context.Context, l *leaseServices) error {
	svc, err := l.svc()
	if err != nil {
		return err
	}
	mine, err := svc.Mine(ctx)
	if err != nil {
		return leaseFailed(ctx, "list", err)
	}
	rows := make([]ownLeaseJSON, 0, len(mine))
	for _, m := range mine {
		rows = append(rows, ownLeaseJSON{
			ID: m.ID, Since: leaseTime(m.Since), ExpiresAt: leaseTime(m.Expires),
			Status: m.Status, Label: m.Holder.Label,
		})
	}
	if out.Format == output.FormatJSON {
		return writeLeaseJSON(rows)
	}
	if len(rows) == 0 {
		out.WriteMessage("You hold no leases.")
		return nil
	}
	tw := tabwriter.NewWriter(out.Out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tSINCE\tEXPIRES_AT\tSTATUS\tLABEL")
	for _, r := range rows {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", r.ID, r.Since, r.ExpiresAt, r.Status, r.Label)
	}
	return tw.Flush()
}

func leaseTime(t time.Time) string { return t.UTC().Format(time.RFC3339) }

func writeLeaseJSON(v any) error {
	enc := json.NewEncoder(out.Out)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// parseLeaseTTL reads --ttl: empty is zero, which the service reads as its
// default; digits are whole seconds; anything else is a Go duration. Zero and
// values outside [lease.MinTTL, lease.MaxTTL] are refused, so a caller cannot
// mistake 0 for "no expiry".
func parseLeaseTTL(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	var ttl time.Duration
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		if n < int64(lease.MinTTL/time.Second) || n > int64(lease.MaxTTL/time.Second) {
			return 0, errLeaseTTL
		}
		ttl = time.Duration(n) * time.Second
	} else {
		d, err := time.ParseDuration(s)
		if err != nil {
			return 0, errLeaseTTL
		}
		ttl = d
	}
	if ttl < lease.MinTTL || ttl > lease.MaxTTL {
		return 0, errLeaseTTL
	}
	return ttl, nil
}

// readLeaseID returns the lease id from the argument or, with fromStdin,
// from the first line of [leaseStdin]. Exactly one source must be given. A
// terminal on stdin is refused, so the id is never typed where it echoes.
func readLeaseID(arg string, fromStdin bool) (string, error) {
	switch {
	case arg != "" && fromStdin:
		return "", errors.New("give the lease id as an argument or with --lease-id-stdin, not both")
	case fromStdin:
		return readLeaseIDStdin()
	case arg == "":
		return "", errMissingLeaseID
	}
	return strings.TrimSpace(arg), nil
}

var errMissingLeaseID = errors.New("missing lease id: pass it as an argument or with --lease-id-stdin")

func readLeaseIDStdin() (string, error) {
	if f, ok := leaseStdin.(*os.File); ok && isTerminal(f) {
		return "", errors.New("--lease-id-stdin: stdin is a terminal; redirect the lease id from a file")
	}
	sc := bufio.NewScanner(leaseStdin)
	sc.Buffer(make([]byte, 0, maxLeaseIDInput), maxLeaseIDInput)
	if !sc.Scan() {
		if err := sc.Err(); err != nil {
			return "", fmt.Errorf("--lease-id-stdin: %w", err)
		}
		return "", errMissingLeaseID
	}
	id := strings.TrimSpace(sc.Text())
	if id == "" {
		return "", errMissingLeaseID
	}
	return id, nil
}

// leaseClaimFailed answers a failed claim. A miss is the error `rela show`
// gives for the same id, so a claim tells a hidden entity from a missing one
// no better than a read does.
func leaseClaimFailed(ctx context.Context, id string, err error) error {
	if errors.Is(err, store.ErrNotFound) {
		return classifyReadError(id, err)
	}
	return leaseFailed(ctx, "claim", err, "entity", id)
}

// leaseFailed answers a lease error with its [lease.UserText]. An unexpected
// error is logged with the caller's user and tool, plus attrs, and answered
// with [lease.FailedText]. Claim passes the entity id as attrs; renew and
// release know a lease only by its id, which must never be logged.
func leaseFailed(ctx context.Context, op string, err error, attrs ...any) error {
	if text, ok := lease.UserText(err, leaseListHint); ok {
		return &leaseTextError{text: text, err: err}
	}
	p := principal.From(ctx)
	args := append([]any{"op", op, "user", p.User, "tool", p.Tool}, attrs...)
	slog.Warn("cli: lease operation failed", append(args, "err", err)...)
	return errors.New(lease.FailedText)
}
