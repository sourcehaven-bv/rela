// Package tokenstore keeps OAuth refresh tokens for connector scripts and
// hands those scripts short-lived access tokens (TKT-01KZSO).
//
// # The shape
//
// A connection is declared in connections.yaml: where its token endpoint is,
// which secrets hold the client id and secret, and which refresh style the
// provider speaks. An operator stores the refresh token once with
// `rela token set <name>` (or pastes it in the desktop settings). From then
// on a script holding `capabilities.tokens: [<name>]` calls
// rela.oauth.access_token(name) and gets a bearer token, refreshed in Go when
// it is about to expire.
//
// No script ever sees a refresh token, a client secret, or the encryption
// key. A script that leaks its access token leaks something that expires
// within the hour.
//
// # Tiers
//
// [Sealed] encrypts each token with AES-256-GCM into the backend's state
// store, and exists only on the database backends (sqlite, postgres), whose
// state store holds data rather than a node-local cache. The desktop app
// keeps tokens in the OS keychain instead. On the fs and memory backends no
// store is configured and rela.oauth reports `not_configured`.
//
// # Concurrency
//
// A provider that rotates refresh tokens invalidates the old one on use, so
// two processes refreshing at once would leave one of them holding a dead
// token. Every refresh, invalidation, set and delete therefore runs under one
// keyed lock per connection (see [Broker]); on postgres that lock is an
// advisory lock shared by every rela-server process on the schema.
package tokenstore

import (
	"context"
	"errors"
	"time"
)

// Token is what the store keeps for one connection.
type Token struct {
	// Refresh is the long-lived refresh token. Never empty in a stored token.
	Refresh string `json:"refresh"`
	// Access is the current access token, or "" when none was fetched yet or
	// the last one was rejected.
	Access string `json:"access,omitempty"`
	// ExpiresAt is when Access stops working.
	ExpiresAt time.Time `json:"expires_at,omitzero"`
	// NeedsConsentAt records when the provider refused the refresh token.
	// While set, [Broker.AccessToken] fails fast with [ErrNeedsConsent]
	// instead of calling the provider again; `rela token set` clears it.
	NeedsConsentAt time.Time `json:"needs_consent_at,omitzero"`
}

// freshMargin is how long before expiry an access token counts as stale. A
// token handed out with less time left could expire in flight.
const freshMargin = 60 * time.Second

// Fresh reports whether the access token is usable for at least another
// minute at now.
func (t Token) Fresh(now time.Time) bool {
	return t.Access != "" && t.ExpiresAt.Sub(now) > freshMargin
}

// Store keeps tokens by connection name. Implementations: [Sealed] and the
// desktop keychain store. Each must pass tokenstoretest.RunAll.
//
// A Store does no locking of its own: [Broker] serializes every write to a
// name, so a Store needs only per-call atomicity.
type Store interface {
	// Get returns the token for name, or an error wrapping [ErrNotFound]
	// when none is stored.
	Get(ctx context.Context, name Name) (Token, error)
	// Put stores t for name, replacing any earlier token.
	Put(ctx context.Context, name Name, t Token) error
	// Delete removes the token for name. Deleting a missing token is not an
	// error.
	Delete(ctx context.Context, name Name) error
}

var (
	// ErrNotFound reports that no token is stored for a connection.
	ErrNotFound = errors.New("tokenstore: no token stored")
	// ErrNotConfigured reports that the project has no token store, or that
	// connections.yaml does not declare the connection.
	ErrNotConfigured = errors.New("tokenstore: not configured")
	// ErrWrongKey reports a token sealed under a different encryption key.
	ErrWrongKey = errors.New("tokenstore: token was sealed with a different token_key")
	// ErrCorrupt reports a stored token that does not decrypt or decode.
	ErrCorrupt = errors.New("tokenstore: stored token is corrupt")
	// ErrNeedsConsent reports that the provider refused the refresh token. An
	// operator must run the consent flow again and store a new one.
	ErrNeedsConsent = errors.New("tokenstore: the provider refused the refresh token; " +
		"run the consent flow again and store the new token with `rela token set`")
)
