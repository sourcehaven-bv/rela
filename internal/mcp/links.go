package mcp

import (
	"context"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// baseURLKey carries the web app origin of one MCP request on its ctx.
type baseURLKey struct{}

// withBaseURL returns ctx carrying base, the "scheme://host" origin that
// [entityURL] prefixes. An empty base leaves ctx unchanged.
func withBaseURL(ctx context.Context, base string) context.Context {
	if base == "" {
		return ctx
	}
	return context.WithValue(ctx, baseURLKey{}, base)
}

// entityURL returns the web page of the entity row ref of type typ:
// `<base>/entity/<type>/<id>`, or `<id>@<face>` for a faced row. It returns
// "" when ctx carries no base, which is every stdio call, so the field is
// omitted rather than guessed.
//
// The path has no `/s/<space>` prefix. A type can be created from several
// spaces, and the SPA serves the unprefixed route in the default space.
func entityURL(ctx context.Context, typ string, ref entity.Ref) string {
	if ref.IsZero() {
		return ""
	}
	return pageURL(ctx, typ, ref.String())
}

// familyURL returns the web page of entity id as a whole, for a result that
// is entity level and holds no row: a trace node, or an orphan family whose
// world serves no face. The address is the bare id, which the web app
// resolves to a face the way it resolves an id a user types; no store read
// takes it.
func familyURL(ctx context.Context, typ, id string) string {
	if id == "" {
		return ""
	}
	return pageURL(ctx, typ, id)
}

// pageURL joins the ctx base, typ and the address addr into an entity page
// URL, or returns "" without a base or a type.
func pageURL(ctx context.Context, typ, addr string) string {
	base, _ := ctx.Value(baseURLKey{}).(string)
	if base == "" || typ == "" {
		return ""
	}
	return base + "/entity/" + url.PathEscape(typ) + "/" + url.PathEscape(addr)
}

// hostUnsafe matches any character not allowed in a hostname[:port] or a
// bracketed IPv6 literal.
var hostUnsafe = regexp.MustCompile(`[^a-zA-Z0-9.:\[\]-]`)

// requestBaseURL returns the "scheme://host" origin r reached, the same rule
// the data-entry calendar feed uses for its deep links. The proxy in front of
// rela-server forwards the public Host (see [Server.HTTPHandler]). Scheme
// follows TLS or an X-Forwarded-Proto hint. It returns "" when the Host is
// empty or holds characters a host cannot have.
func requestBaseURL(r *http.Request) string {
	if r.Host == "" || hostUnsafe.MatchString(r.Host) {
		return ""
	}
	scheme := "http"
	if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}

// withRequestBaseURL wraps next so every tool call in a request sees the
// request's origin through [entityURL].
func withRequestBaseURL(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r.WithContext(withBaseURL(r.Context(), requestBaseURL(r))))
	})
}
