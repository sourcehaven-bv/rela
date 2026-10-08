package tokenstore

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"regexp"
	"strings"
	"unicode"

	"gopkg.in/yaml.v3"
)

// ConnectionsFile is the project-root file declaring connections. It is
// operator config, like schedules.yaml: it names secrets but holds none.
const ConnectionsFile = "connections.yaml"

// Style is how a provider expects a refresh request.
type Style string

const (
	// StyleRFC6749 posts grant_type=refresh_token and the refresh token as a
	// form body and authenticates the client with HTTP Basic (RFC 6749
	// section 6).
	StyleRFC6749 Style = "rfc6749"
	// StyleLaunchpad posts the same form body with client_id and
	// client_secret as form fields instead of HTTP Basic. 37signals
	// Launchpad (Basecamp) needs this.
	StyleLaunchpad Style = "launchpad"
)

// Connection is one declared OAuth connection.
type Connection struct {
	Name Name
	// TokenURL is the provider's token endpoint. https, or http to a
	// loopback address.
	TokenURL string
	// ClientIDSecret and ClientSecretSecret name the global secrets holding
	// the client credentials. The values never appear in config.
	ClientIDSecret     string
	ClientSecretSecret string
	Style              Style
	// UserAgent is sent on refresh requests. Some providers (Basecamp)
	// refuse a request without one that names a contact.
	UserAgent string
}

// Connections is the parsed connections.yaml, by name.
type Connections map[Name]Connection

type rawConnection struct {
	TokenURL           string `yaml:"token_url"`
	ClientIDSecret     string `yaml:"client_id_secret"`
	ClientSecretSecret string `yaml:"client_secret_secret"`
	Style              Style  `yaml:"style"`
	UserAgent          string `yaml:"user_agent"`
}

type rawConnections struct {
	Connections map[string]rawConnection `yaml:"connections"`
}

// secretNamePattern is the secret name grammar the desktop keychain enforces.
// Holding config to it means a connection works on every tier.
var secretNamePattern = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,64}$`)

// ParseConnections parses connections.yaml. Unknown keys are errors, so a
// misspelled field cannot silently fall back to a default.
func ParseConnections(data []byte) (Connections, error) {
	var raw rawConnections
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	// io.EOF is an empty file: no connections.
	if err := dec.Decode(&raw); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("%s: %w", ConnectionsFile, err)
	}
	out := make(Connections, len(raw.Connections))
	for rawName, rc := range raw.Connections {
		name, err := ParseName(rawName)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", ConnectionsFile, err)
		}
		c := Connection{
			Name: name, TokenURL: rc.TokenURL, ClientIDSecret: rc.ClientIDSecret,
			ClientSecretSecret: rc.ClientSecretSecret, Style: rc.Style, UserAgent: rc.UserAgent,
		}
		if err := c.validate(); err != nil {
			return nil, fmt.Errorf("%s: connection %q: %w", ConnectionsFile, name, err)
		}
		out[name] = c
	}
	return out, nil
}

func (c Connection) validate() error {
	if err := validateTokenURL(c.TokenURL); err != nil {
		return err
	}
	for field, name := range map[string]string{
		"client_id_secret": c.ClientIDSecret, "client_secret_secret": c.ClientSecretSecret,
	} {
		if !secretNamePattern.MatchString(name) {
			return fmt.Errorf("%s must name a secret (1-64 letters, digits, '_', '.' or '-'), got %q", field, name)
		}
		if name == KeySecret {
			return fmt.Errorf("%s may not name %q, the token store key", field, KeySecret)
		}
	}
	switch c.Style {
	case StyleRFC6749, StyleLaunchpad:
	default:
		return fmt.Errorf("style must be %q or %q, got %q", StyleRFC6749, StyleLaunchpad, c.Style)
	}
	if strings.ContainsFunc(c.UserAgent, unicode.IsControl) {
		return errors.New("user_agent must not contain control characters")
	}
	return nil
}

// validateTokenURL accepts an absolute https URL, or http to a loopback
// address (a local development server, a test stub). It refuses userinfo
// and a fragment.
func validateTokenURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return fmt.Errorf("token_url must be an absolute URL, got %q", raw)
	}
	if u.User != nil || u.Fragment != "" {
		return errors.New("token_url must not carry userinfo or a fragment")
	}
	switch u.Scheme {
	case "https":
		return nil
	case "http":
		if isLoopback(u.Hostname()) {
			return nil
		}
		return fmt.Errorf("token_url must use https (http only to a loopback address), got %q", raw)
	default:
		return fmt.Errorf("token_url must use https, got %q", raw)
	}
}

func isLoopback(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
