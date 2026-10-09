package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/tokenstore"
)

// Connector tokens on the desktop (TKT-01KZSO).
//
// A token is two keychain items beside the document's secrets:
// "token/<name>/refresh" holds the refresh token and "token/<name>/state"
// the access token, its expiry and any needs-consent mark. Two items,
// because each platform caps an item's size and a provider's tokens can
// each approach it.
//
// They go through keychainSecrets, so they get its place check: a document
// that only carries this document's ID cannot read or replace them. A
// secret name cannot contain '/', so no secret can collide with them, and
// they are kept out of the secrets scripts read and the settings list.

// tokenItemPrefix starts every token item's name.
const tokenItemPrefix = "token/"

// errTokensNotTrusted refuses token access at a place that may not read
// the document's keychain items.
var errTokensNotTrusted = fmt.Errorf("connector tokens are not trusted at this location: %w", errNotTrusted)

// keychainTokens is a tokenstore.Store over one document's keychain items.
type keychainTokens struct {
	kc    *keychainSecrets
	docID string
	place string
}

// newKeychainTokens is the [appbuild.WithTokenStore] source: it stores the
// tokens of the project whose host config is host.
func newKeychainTokens(host appbuild.HostConfig) (tokenstore.Store, error) {
	h, ok := host.(*desktopHost)
	if !ok || h == nil {
		return nil, errors.New("desktop tokens: the project has no keychain host config")
	}
	return &keychainTokens{kc: h.secrets, docID: h.docID, place: h.Path()}, nil
}

// tokenState is the JSON in a token's state item.
type tokenState struct {
	Access         string    `json:"access,omitempty"`
	ExpiresAt      time.Time `json:"expires_at,omitzero"`
	NeedsConsentAt time.Time `json:"needs_consent_at,omitzero"`
}

func refreshItem(n tokenstore.Name) string { return tokenItemPrefix + string(n) + "/refresh" }
func stateItem(n tokenstore.Name) string   { return tokenItemPrefix + string(n) + "/state" }

// Get reads the token; tokenstore.ErrNotFound when none is stored.
func (s *keychainTokens) Get(_ context.Context, n tokenstore.Name) (tokenstore.Token, error) {
	values, trusted, err := s.kc.forPlace(s.docID, s.place)
	if err != nil {
		return tokenstore.Token{}, err
	}
	if !trusted {
		return tokenstore.Token{}, errTokensNotTrusted
	}
	refresh, ok := values[refreshItem(n)]
	if !ok {
		return tokenstore.Token{}, tokenstore.ErrNotFound
	}
	t := tokenstore.Token{Refresh: refresh}
	if raw, ok := values[stateItem(n)]; ok {
		var st tokenState
		if err := json.Unmarshal([]byte(raw), &st); err != nil {
			return tokenstore.Token{}, tokenstore.ErrCorrupt
		}
		t.Access, t.ExpiresAt, t.NeedsConsentAt = st.Access, st.ExpiresAt, st.NeedsConsentAt
	}
	return t, nil
}

// Put stores the token. The refresh item is written first. A provider
// that rotates refresh tokens has already revoked the old one, so the new
// refresh token is the item that must not be lost. If the state write
// then fails, the stored access token is out of date; the broker
// refreshes it once it expires or a call is refused.
func (s *keychainTokens) Put(_ context.Context, n tokenstore.Name, t tokenstore.Token) error {
	if t.Refresh == "" {
		return errors.New("desktop tokens: a refresh token is required")
	}
	raw, err := json.Marshal(tokenState{Access: t.Access, ExpiresAt: t.ExpiresAt, NeedsConsentAt: t.NeedsConsentAt})
	if err != nil {
		return err
	}
	if err := s.kc.put(s.docID, s.place, refreshItem(n), t.Refresh); err != nil {
		return err
	}
	return s.kc.put(s.docID, s.place, stateItem(n), string(raw))
}

// Delete removes both items. Deleting a token that is not stored succeeds.
func (s *keychainTokens) Delete(_ context.Context, n tokenstore.Name) error {
	if err := s.kc.remove(s.docID, s.place, refreshItem(n)); err != nil {
		return err
	}
	return s.kc.remove(s.docID, s.place, stateItem(n))
}

// withoutTokenItems returns values without the token items, which are not
// secrets a script may read.
func withoutTokenItems(values map[string]string) map[string]string {
	out := maps.Clone(values)
	maps.DeleteFunc(out, func(k, _ string) bool { return strings.HasPrefix(k, tokenItemPrefix) })
	return out
}
