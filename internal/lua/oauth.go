// Lua bindings for OAuth access tokens (TKT-01KZSO).
//
//	rela.oauth.access_token(name)          -> token | nil, err
//	rela.oauth.invalidate(name, rejected)  -> true  | nil, err
//
// access_token returns a bearer token for the connection name, refreshed in
// Go when it is about to expire. invalidate tells rela the API answered 401
// with the token rejected, so the next access_token refreshes; it clears the
// stored token only if it is still the rejected one.
//
// A script never sees a refresh token or a client secret: those stay in the
// token store and the refresher. A leaked access token expires within the
// hour.
//
// err is a table like http's: kind, message, retry_after (always 0),
// details. Kinds:
//   - denied: the script's `capabilities.tokens` does not name the connection
//   - not_configured: the project has no token store, the connection is not
//     in connections.yaml, or no token is stored for it
//   - needs_consent: the provider refused the refresh token; an operator
//     must run the consent flow and `rela token set` again
//   - error: anything else (the provider is down, the store failed)
//
// rela.oauth exists only on a writer runtime whose grant names a
// connection. Calling it inside a store transaction raises: a refresh is
// slow network I/O, and a transaction must not wait on it.
//
// Free functions and a bindings type rather than Runtime methods: Runtime is
// at its plimsoll method cap.
package lua

import (
	"context"
	"errors"

	lua "github.com/yuin/gopher-lua"

	"github.com/Sourcehaven-BV/rela/internal/store"
)

// OAuthTokens hands out access tokens. Defined here at the consumer;
// appbuild adapts the token store's broker to it and translates its errors
// to [ErrOAuthNotConfigured] and [ErrOAuthNeedsConsent].
type OAuthTokens interface {
	AccessToken(ctx context.Context, name string) (string, error)
	Invalidate(ctx context.Context, name, rejected string) error
}

var (
	// ErrOAuthNotConfigured marks an error the binding reports as kind
	// not_configured.
	ErrOAuthNotConfigured = errors.New("oauth: not configured")
	// ErrOAuthNeedsConsent marks an error the binding reports as kind
	// needs_consent.
	ErrOAuthNeedsConsent = errors.New("oauth: needs consent")

	errOAuthNoStore = errors.New("no token store is configured for this project: tokens need the " +
		"sqlite or postgres backend (or the desktop app) and a token_key")
	errOAuthDenied = errors.New("connection not granted: add it to this script's `capabilities.tokens`")
)

// oauthBindings holds the runtime the bindings run in.
type oauthBindings struct{ r *Runtime }

// registerOAuthModule installs rela.oauth on a writer runtime whose grant
// names a connection.
func registerOAuthModule(r *Runtime, rela *lua.LTable) {
	if !r.caps.anyTokens() {
		return
	}
	ob := oauthBindings{r: r}
	tbl := r.L.NewTable()
	r.L.SetField(tbl, "access_token", r.L.NewFunction(ob.luaAccessToken))
	r.L.SetField(tbl, "invalidate", r.L.NewFunction(ob.luaInvalidate))
	r.L.SetField(rela, "oauth", tbl)
}

// begin runs the checks both bindings share and returns the token source,
// or pushes the error and returns ok false.
func (ob oauthBindings) begin(ls *lua.LState, binding, name string) (OAuthTokens, context.Context, bool) {
	ctx := ob.r.callerCtx()
	if store.InTx(ctx) {
		ls.RaiseError("%s: cannot run inside a store transaction; call it from a background "+
			"automation or a scheduled script", binding)
		return nil, nil, false
	}
	if !ob.r.caps.AllowsToken(name) {
		pushOAuthError(ls, "denied", errOAuthDenied.Error())
		return nil, nil, false
	}
	if ob.r.deps.OAuth == nil {
		pushOAuthError(ls, "not_configured", errOAuthNoStore.Error())
		return nil, nil, false
	}
	return ob.r.deps.OAuth, ctx, true
}

func (ob oauthBindings) luaAccessToken(ls *lua.LState) int {
	name := ls.CheckString(1)
	src, ctx, ok := ob.begin(ls, "rela.oauth.access_token", name)
	if !ok {
		return 2
	}
	tok, err := src.AccessToken(ctx, name)
	if err != nil {
		return pushOAuthError(ls, oauthErrorKind(err), err.Error())
	}
	ls.Push(lua.LString(tok))
	return 1
}

func (ob oauthBindings) luaInvalidate(ls *lua.LState) int {
	name := ls.CheckString(1)
	rejected := ls.CheckString(2)
	src, ctx, ok := ob.begin(ls, "rela.oauth.invalidate", name)
	if !ok {
		return 2
	}
	if err := src.Invalidate(ctx, name, rejected); err != nil {
		return pushOAuthError(ls, oauthErrorKind(err), err.Error())
	}
	ls.Push(lua.LTrue)
	return 1
}

func oauthErrorKind(err error) string {
	switch {
	case errors.Is(err, ErrOAuthNotConfigured):
		return "not_configured"
	case errors.Is(err, ErrOAuthNeedsConsent):
		return "needs_consent"
	default:
		return "error"
	}
}

// pushOAuthError pushes (nil, err_table) in the shape mail.send and http use.
func pushOAuthError(ls *lua.LState, kind, message string) int {
	return pushMailError(ls, kind, message)
}
