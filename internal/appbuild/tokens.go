package appbuild

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"regexp"

	"github.com/Sourcehaven-BV/rela/internal/config"
	"github.com/Sourcehaven-BV/rela/internal/lock"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/secrets"
	"github.com/Sourcehaven-BV/rela/internal/state"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/tokenstore"
)

// WithTokenStore replaces the sealed database store as where connector
// refresh tokens live (TKT-01KZSO). build runs once per assembled store, with
// that store's host config. The desktop uses it to keep tokens in the OS
// keychain, on every backend.
func WithTokenStore(build func(HostConfig) (tokenstore.Store, error)) Option {
	return func(o *options) { o.tokenStore = build }
}

// tokenScopeKey is the state key holding a sqlite project's token scope.
// Outside the `tokens/` prefix, so no connection name can reach it.
const tokenScopeKey = "token-scope"

var tokenScopePattern = regexp.MustCompile(`^[0-9a-f]{32}$`)

// tokenSetup is the outcome of wiring the token store for one assembly.
type tokenSetup struct {
	broker *tokenstore.Broker
	// err says why broker is nil. It wraps tokenstore.ErrNotConfigured.
	err error
}

// buildTokens wires the token store and its broker for one assembled store.
// A project without a token store is not an error: the broker is nil and
// err says why, and rela.oauth and `rela token` report that reason.
//
// The store is the [WithTokenStore] source when set; otherwise a
// [tokenstore.Sealed] over backendKV, which only the database backends
// supply. On fs and memory there is no token store (see the tier table in
// the Lua scripting guide).
//
// The connections come from the base, which refused an invalid
// connections.yaml when it was prepared.
//
// locker is the Services-wide lock shared with attachments: one Locker per
// store, never one per runtime, so two refreshes of one connection exclude
// each other.
func buildTokens(
	ctx context.Context, base *SharedBase, st store.Store, backendKV state.KV,
	host lua.HostConfig, locker lock.Locker,
) tokenSetup {
	conns := base.conns
	var (
		tokStore tokenstore.Store
		scope    string
		err      error
	)
	switch {
	case base.opts.tokenStore != nil:
		tokStore, err = base.opts.tokenStore(host)
		if err != nil {
			return notConfigured(err)
		}
		scope = base.cfg.Paths.Root
	case backendKV != nil:
		global, gerr := host.Secrets("")
		if gerr != nil && !errors.Is(gerr, secrets.ErrNotFound) {
			return notConfigured(fmt.Errorf("read secrets: %w", gerr))
		}
		key, kerr := tokenstore.LoadKey(global, os.Getenv)
		if kerr != nil {
			if !errors.Is(kerr, tokenstore.ErrNoKey) || len(conns) > 0 {
				slog.Warn("token store disabled", "error", kerr)
			}
			return notConfigured(kerr)
		}
		scope, err = tokenScope(ctx, st, backendKV)
		if err != nil {
			return notConfigured(err)
		}
		tokStore, err = tokenstore.NewSealed(backendKV, key, scope)
		if err != nil {
			return notConfigured(err)
		}
	default:
		return notConfigured(errors.New("tokens need the sqlite or postgres backend, or the desktop app"))
	}
	b, err := tokenstore.NewBroker(tokenstore.BrokerDeps{
		Store: tokStore, Locker: locker, Secrets: host, Connections: conns, Scope: scope,
	})
	if err != nil {
		return notConfigured(err)
	}
	return tokenSetup{broker: b}
}

func notConfigured(reason error) tokenSetup {
	return tokenSetup{err: fmt.Errorf("%w: %w", tokenstore.ErrNotConfigured, reason)}
}

// loadConnections reads connections.yaml through the project config loader,
// so a sqlite project may carry it in its database. An absent file declares
// no connections; a file that does not parse or validate is an error.
func loadConnections(ctx context.Context, cfgLoader config.Loader) (tokenstore.Connections, error) {
	data, err := cfgLoader.Load(ctx, tokenstore.ConnectionsFile)
	if errors.Is(err, fs.ErrNotExist) || os.IsNotExist(err) {
		return tokenstore.Connections{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", tokenstore.ConnectionsFile, err)
	}
	return tokenstore.ParseConnections(data)
}

// tokenScope names the tenant a sealed token belongs to: the schema on
// postgres, so every process serving it agrees without coordination, and an
// id minted once into the state store on sqlite, which is single-process.
func tokenScope(ctx context.Context, st store.Store, kv state.KV) (string, error) {
	if schema, ok, err := backendTokenScope(ctx, st); ok || err != nil {
		return schema, err
	}
	raw, err := kv.Get(ctx, tokenScopeKey)
	if err == nil && tokenScopePattern.Match(raw) {
		return string(raw), nil
	}
	if err != nil && !os.IsNotExist(err) && !errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("read token scope: %w", err)
	}
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil { // coverage-ignore: crypto/rand does not fail
		return "", err
	}
	id := hex.EncodeToString(buf)
	if err := kv.Put(ctx, tokenScopeKey, []byte(id)); err != nil {
		return "", fmt.Errorf("store token scope: %w", err)
	}
	return id, nil
}

// Tokens returns the token broker (TKT-01KZSO), or an error wrapping
// [tokenstore.ErrNotConfigured] that says why the project has none. A
// package function because Services sits at its plimsoll line.
func Tokens(s *Services) (*tokenstore.Broker, error) {
	if s.tokens.broker == nil {
		if s.tokens.err != nil {
			return nil, s.tokens.err
		}
		return nil, fmt.Errorf("%w: no token store was assembled", tokenstore.ErrNotConfigured)
	}
	return s.tokens.broker, nil
}

// ScriptOAuth returns what rela.oauth reads tokens through, or nil when the
// project has no token store, so the binding reports not_configured.
func ScriptOAuth(s *Services) lua.OAuthTokens {
	if s.tokens.broker == nil {
		return nil
	}
	return scriptOAuth{b: s.tokens.broker}
}

// scriptOAuth adapts the broker to lua.OAuthTokens, translating its errors
// into the binding's kinds.
type scriptOAuth struct{ b *tokenstore.Broker }

func (o scriptOAuth) AccessToken(ctx context.Context, name string) (string, error) {
	tok, err := o.b.AccessToken(ctx, name)
	return tok, translateTokenError(err)
}

func (o scriptOAuth) Invalidate(ctx context.Context, name, rejected string) error {
	return translateTokenError(o.b.Invalidate(ctx, name, rejected))
}

func translateTokenError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, tokenstore.ErrNeedsConsent):
		return fmt.Errorf("%w: %w", lua.ErrOAuthNeedsConsent, err)
	case errors.Is(err, tokenstore.ErrNotConfigured), errors.Is(err, tokenstore.ErrNotFound):
		return fmt.Errorf("%w: %w", lua.ErrOAuthNotConfigured, err)
	default:
		return err
	}
}
