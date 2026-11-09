package httpremote

import (
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
)

// Authenticator adds the credential to a request's headers. The credential
// comes from the remote's identity edge, such as a machine token from its
// OIDC provider; rela issues none of its own (DEC-QKLMMD4).
type Authenticator interface {
	Authenticate(h http.Header)
}

// token is a credential that never prints its value.
type token struct {
	header string
	prefix string
	value  string
}

// Bearer returns an Authenticator that sends "Authorization: Bearer
// <value>".
//
// A value that already starts with "Bearer " is sent once, not twice.
func Bearer(value string) (Authenticator, error) {
	value = strings.TrimSpace(value)
	switch {
	case strings.EqualFold(value, strings.TrimSpace(bearerPrefix)):
		value = ""
	case len(value) > len(bearerPrefix) && strings.EqualFold(value[:len(bearerPrefix)], bearerPrefix):
		value = value[len(bearerPrefix):]
	}
	return newToken("Authorization", bearerPrefix, value)
}

const bearerPrefix = "Bearer "

// reservedHeaders cannot carry a credential: the transport sets them, or a
// hop strips them.
var reservedHeaders = []string{
	"Connection", "Cookie", "Host", "Keep-Alive", "Proxy-Authorization", "Proxy-Connection", "Te", "Trailer",
	"Transfer-Encoding", "Upgrade",
}

// Header returns an Authenticator that sends value as is in header name,
// for a deployment whose gate reads another header. Name "Authorization"
// sends a bearer token.
func Header(name, value string) (Authenticator, error) {
	if strings.EqualFold(name, "Authorization") {
		return Bearer(value)
	}
	if !validHeaderName(name) {
		return nil, fmt.Errorf("httpremote: %q is not a valid header name", name)
	}
	switch c := http.CanonicalHeaderKey(name); {
	case slices.Contains(reservedHeaders, c), strings.HasPrefix(c, "Content-"):
		return nil, fmt.Errorf("httpremote: header %q cannot carry a credential", name)
	}
	return newToken(http.CanonicalHeaderKey(name), "", value)
}

func newToken(header, prefix, value string) (Authenticator, error) {
	value = strings.TrimSpace(value)
	switch {
	case value == "":
		return nil, errors.New("httpremote: the credential is empty")
	case !validHeaderValue(value):
		return nil, errors.New("httpremote: the credential holds a character a header cannot carry")
	}
	return token{header: header, prefix: prefix, value: value}, nil
}

// Authenticate sets the header.
func (t token) Authenticate(h http.Header) { h.Set(t.header, t.prefix+t.value) }

// String names the header, never the value.
func (t token) String() string { return t.header + ": [redacted]" }

// GoString is String, so %#v does not print the value either.
func (t token) GoString() string { return t.String() }

// validHeaderName reports whether name is an RFC 9110 token.
func validHeaderName(name string) bool {
	if name == "" {
		return false
	}
	for i := range len(name) {
		c := name[i]
		alnum := c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
		if !alnum && !strings.ContainsRune("!#$%&'*+-.^_`|~", rune(c)) {
			return false
		}
	}
	return true
}

// validHeaderValue reports whether v holds only visible ASCII and spaces, so
// it cannot split a header.
func validHeaderValue(v string) bool {
	for i := range len(v) {
		if c := v[i]; c < ' ' || c > '~' {
			return false
		}
	}
	return true
}
