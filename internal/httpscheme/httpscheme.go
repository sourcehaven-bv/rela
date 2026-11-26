// Package httpscheme names the two URL schemes rela treats as web
// addresses, so each caller that validates an outbound or configured URL
// tests the same set.
package httpscheme

// The URL schemes of a web address, as net/url reports them.
const (
	HTTP  = "http"
	HTTPS = "https"
)

// IsHTTP reports whether scheme is http or https. It compares exactly, so
// the caller lowercases first when the scheme was not parsed by net/url.
func IsHTTP(scheme string) bool {
	return scheme == HTTP || scheme == HTTPS
}
