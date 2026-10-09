package tokenstore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"time"
)

// Locker is the keyed lock the broker serializes a connection's writes
// under. lock.Locker satisfies it; the wiring site passes the one Locker
// shared by every service of a store, so the attachment service and the
// broker draw from one pool.
type Locker interface {
	Acquire(ctx context.Context, key string) (release func(), err error)
}

// SecretSource reads the project's secrets. lua.HostConfig satisfies it. The
// broker asks for the global secrets ("") on every refresh, so a client
// secret rotated in secrets.yaml or the keychain takes effect without a
// restart.
type SecretSource interface {
	Secrets(scriptPath string) (map[string]string, error)
}

// Doer sends an HTTP request. *http.Client satisfies it.
type Doer interface {
	Do(req *http.Request) (*http.Response, error)
}

const (
	// lockWait bounds how long a caller waits for another refresh of the
	// same connection. A refresh takes one HTTP round trip; a minute means
	// the holder is stuck, and the waiter should fail rather than queue.
	lockWait = 60 * time.Second
	// refreshTimeout bounds the token endpoint call.
	refreshTimeout = 30 * time.Second
	// persistTimeout bounds one attempt to store a refreshed token.
	persistTimeout = 10 * time.Second
	// defaultExpiry applies when the provider names no lifetime.
	defaultExpiry = time.Hour
	// maxExpiry caps the lifetime a provider may name. A larger value is
	// read as this one, so a huge expires_in cannot overflow the duration
	// or keep an access token for ever.
	maxExpiry = 365 * 24 * time.Hour
)

// BrokerDeps are the broker's collaborators.
type BrokerDeps struct {
	// Store keeps the tokens. Required.
	Store Store
	// Locker serializes refresh, invalidate, set and delete per connection.
	// Required.
	Locker Locker
	// Secrets holds the client credentials connections.yaml names. Required.
	Secrets SecretSource
	// Connections is the parsed connections.yaml.
	//
	// Nil: accepted; no connection is declared, so every call reports
	// [ErrNotConfigured].
	Connections Connections
	// Scope keeps lock keys of two tenants sharing a database apart. The
	// same value as the [Sealed] scope. Required.
	Scope string
	// HTTP sends token requests.
	//
	// Nil: accepted; a client that follows no redirects and times out after
	// 30 seconds is used.
	HTTP Doer
	// Now is the clock.
	//
	// Nil: accepted; time.Now is used.
	Now func() time.Time
}

// Broker hands out access tokens and refreshes them in Go (TKT-01KZSO). It
// is the only code that reads a refresh token or a client secret; scripts
// reach it through rela.oauth and see access tokens only.
type Broker struct {
	store     Store
	locker    Locker
	secrets   SecretSource
	conns     Connections
	lockScope string
	http      Doer
	now       func() time.Time
	lockWait  time.Duration
}

// NewBroker returns a broker over d.
func NewBroker(d BrokerDeps) (*Broker, error) {
	switch {
	case d.Store == nil:
		return nil, errors.New("tokenstore: NewBroker needs a Store")
	case d.Locker == nil:
		return nil, errors.New("tokenstore: NewBroker needs a Locker")
	case d.Secrets == nil:
		return nil, errors.New("tokenstore: NewBroker needs a SecretSource")
	case d.Scope == "":
		return nil, errors.New("tokenstore: NewBroker needs a Scope")
	}
	b := &Broker{
		store: d.Store, locker: d.Locker, secrets: d.Secrets, conns: d.Connections,
		http: d.HTTP, now: d.Now, lockWait: lockWait,
	}
	sum := sha256.Sum256([]byte(d.Scope))
	b.lockScope = hex.EncodeToString(sum[:8])
	if b.http == nil {
		b.http = &http.Client{
			Timeout: refreshTimeout,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
	}
	if b.now == nil {
		b.now = time.Now
	}
	return b, nil
}

// Connections returns a copy of the declared connections.
func (b *Broker) Connections() Connections { return maps.Clone(b.conns) }

// connection validates name and returns its declaration.
func (b *Broker) connection(name string) (Connection, error) {
	n, err := ParseName(name)
	if err != nil {
		return Connection{}, err
	}
	c, ok := b.conns[n]
	if !ok {
		return Connection{}, fmt.Errorf("%w: connection %q is not declared in %s", ErrNotConfigured, n, ConnectionsFile)
	}
	return c, nil
}

// lock takes the connection's lock, waiting at most lockWait. The returned
// release must be deferred.
func (b *Broker) lock(ctx context.Context, n Name) (func(), error) {
	wctx, cancel := context.WithTimeout(ctx, b.lockWait)
	defer cancel()
	release, err := b.locker.Acquire(wctx, "oauth-refresh/"+b.lockScope+"/"+string(n))
	if err != nil {
		return nil, fmt.Errorf("tokenstore: waiting for another refresh of %q: %w", n, err)
	}
	return release, nil
}

// load reads the token for n and refuses one marked as needing consent.
func (b *Broker) load(ctx context.Context, n Name) (Token, error) {
	t, err := b.store.Get(ctx, n)
	if errors.Is(err, ErrNotFound) {
		return Token{}, fmt.Errorf("%w; store one with `rela token set %s`", err, n)
	}
	if err != nil {
		return Token{}, err
	}
	if !t.NeedsConsentAt.IsZero() {
		return Token{}, fmt.Errorf("%w (%q, since %s)", ErrNeedsConsent, n, t.NeedsConsentAt.UTC().Format(time.RFC3339))
	}
	return t, nil
}

// AccessToken returns a usable access token for the connection name,
// refreshing it when it expires within a minute.
//
// Only one refresh per connection runs at a time, across processes on
// postgres. A caller that finds a refresh in progress waits for it and then
// uses its result.
func (b *Broker) AccessToken(ctx context.Context, name string) (string, error) {
	conn, err := b.connection(name)
	if err != nil {
		return "", err
	}
	t, err := b.load(ctx, conn.Name)
	if err != nil {
		return "", err
	}
	if t.Fresh(b.now()) {
		return t.Access, nil
	}

	release, err := b.lock(ctx, conn.Name)
	if err != nil {
		return "", err
	}
	defer release()

	// Another holder may have refreshed while this caller waited.
	t, err = b.load(ctx, conn.Name)
	if err != nil {
		return "", err
	}
	if t.Fresh(b.now()) {
		return t.Access, nil
	}
	if cerr := ctx.Err(); cerr != nil {
		return "", cerr
	}

	next, err := b.refresh(ctx, conn, t)
	if errors.Is(err, errInvalidGrant) {
		t.Access, t.ExpiresAt, t.NeedsConsentAt = "", time.Time{}, b.now()
		if perr := b.persist(ctx, conn.Name, t); perr != nil {
			return "", errors.Join(fmt.Errorf("%w (%q)", ErrNeedsConsent, conn.Name), perr)
		}
		return "", fmt.Errorf("%w (%q)", ErrNeedsConsent, conn.Name)
	}
	if err != nil {
		return "", err
	}
	if err := b.persist(ctx, conn.Name, next); err != nil {
		return "", err
	}
	return next.Access, nil
}

// persist stores t on a context the caller cannot cancel, with one retry. A
// provider that rotates refresh tokens has already revoked the old one, so
// a refreshed token that is not stored is a lost connection.
func (b *Broker) persist(ctx context.Context, n Name, t Token) error {
	ctx = context.WithoutCancel(ctx)
	var err error
	for range 2 {
		pctx, cancel := context.WithTimeout(ctx, persistTimeout)
		err = b.store.Put(pctx, n, t)
		cancel()
		if err == nil {
			return nil
		}
	}
	return fmt.Errorf("tokenstore: storing the refreshed token for %q failed: %w", n, err)
}

// Invalidate clears the stored access token of the connection name if it
// still equals rejected, so the next AccessToken refreshes. A script calls
// it after the API answered 401 with that token.
//
// Compare-and-clear under the connection's lock: when two scripts both saw
// a 401 for the same token, the second must not clear the token the first
// one's refresh just stored.
func (b *Broker) Invalidate(ctx context.Context, name, rejected string) error {
	conn, err := b.connection(name)
	if err != nil {
		return err
	}
	release, err := b.lock(ctx, conn.Name)
	if err != nil {
		return err
	}
	defer release()
	t, err := b.store.Get(ctx, conn.Name)
	if err != nil {
		return err
	}
	if rejected == "" || t.Access != rejected {
		return nil
	}
	t.Access, t.ExpiresAt = "", time.Time{}
	return b.persist(ctx, conn.Name, t)
}

// Set stores t for the connection name under its lock and clears any
// needs-consent mark. `rela token set` and the desktop settings call it.
func (b *Broker) Set(ctx context.Context, name string, t Token) error {
	conn, err := b.connection(name)
	if err != nil {
		return err
	}
	if t.Refresh == "" {
		return errors.New("tokenstore: a refresh token is required")
	}
	release, err := b.lock(ctx, conn.Name)
	if err != nil {
		return err
	}
	defer release()
	t.NeedsConsentAt = time.Time{}
	return b.store.Put(ctx, conn.Name, t)
}

// Delete removes the stored token for the connection name under its lock.
// It works for an undeclared but valid name, so a connection removed from
// connections.yaml can still be cleaned up.
func (b *Broker) Delete(ctx context.Context, name string) error {
	n, err := ParseName(name)
	if err != nil {
		return err
	}
	release, err := b.lock(ctx, n)
	if err != nil {
		return err
	}
	defer release()
	return b.store.Delete(ctx, n)
}

// Status describes one connection's stored token without its values.
type Status struct {
	Name     Name
	Declared bool
	// Stored is true when a token decrypted and decoded.
	Stored bool
	// AccessFresh is true when the stored access token is usable now.
	AccessFresh    bool
	ExpiresAt      time.Time
	NeedsConsentAt time.Time
	// Err is why the token could not be read, when it exists but does not
	// open ([ErrWrongKey], [ErrCorrupt]) or the store failed. Nil when
	// Stored, and nil when no token is stored.
	Err error
}

// Status reports the state of the connection name. It never returns token
// values.
func (b *Broker) Status(ctx context.Context, name string) (Status, error) {
	n, err := ParseName(name)
	if err != nil {
		return Status{}, err
	}
	_, declared := b.conns[n]
	st := Status{Name: n, Declared: declared}
	t, err := b.store.Get(ctx, n)
	switch {
	case errors.Is(err, ErrNotFound):
	case err != nil:
		st.Err = err
	default:
		st.Stored = true
		st.AccessFresh = t.Fresh(b.now())
		st.ExpiresAt = t.ExpiresAt
		st.NeedsConsentAt = t.NeedsConsentAt
	}
	return st, nil
}
