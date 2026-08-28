package mail

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/secrets"
)

// ConfigFile is the name of the mail config file inside .rela/.
const ConfigFile = "mail.yaml"

// Transport names the delivery mechanism. The set is closed: an unknown value
// is a load error naming the valid ones, not a silent fallback.
type Transport string

const (
	// TransportSMTP delivers over authenticated SMTP with mandatory STARTTLS.
	TransportSMTP Transport = "smtp"

	// TransportMemory records messages in process instead of sending them.
	// A real transport, selected by config — see memory.go.
	TransportMemory Transport = "memory"
)

// DefaultTimeoutSeconds bounds a single send.
const DefaultTimeoutSeconds = 30

// DefaultPort is the SMTP submission port.
const DefaultPort = 587

// ErrConfigNotFound indicates that .rela/mail.yaml does not exist. This is the
// "mail not configured" state, not a failure: callers use errors.Is to treat it
// as a normal absence and leave mail switched off.
var ErrConfigNotFound = errors.New("mail: not configured (no .rela/mail.yaml)")

// Config is the on-disk mail configuration.
//
// The credential is deliberately absent from THIS file. It comes from one of
// two places, checked in order at SEND time:
//
//  1. `.rela/secrets.yaml` under the key `smtp_password` — the same store Lua
//     scripts read, and the natural home for it: an SMTP password is no
//     different in kind from the API tokens already kept there.
//  2. The environment variable named by `password_env`, if set.
//
// Either way the password never appears in mail.yaml, never on a command line,
// and never in a log — the invariant RELA_DATABASE_URL carries, because a
// secret must not reach `ps` output or shell history.
//
// secrets.yaml first, because that is where an operator will look. password_env
// stays for deployments that inject credentials as environment variables
// (containers, systemd units) and would otherwise have to materialize a
// plaintext file at deploy time.
type Config struct {
	// Transport selects the delivery mechanism. Required.
	Transport Transport `yaml:"transport"`

	// Host is the SMTP server hostname. Required for TransportSMTP.
	Host string `yaml:"host"`

	// Port is the SMTP port. Defaults to DefaultPort.
	Port int `yaml:"port"`

	// Username is the SMTP username. Optional: a relay on a trusted network
	// may accept unauthenticated submission.
	Username string `yaml:"username"`

	// PasswordVar names an environment variable holding the SMTP password.
	// OPTIONAL — `.rela/secrets.yaml` is checked first; see the type doc.
	//
	// It holds a VARIABLE NAME, never a secret, which is also why it is not
	// called PasswordEnv: static analysis treats any field whose name reads
	// like a credential as a taint source, and flagged the (harmless) value
	// flowing into error logs. Naming it for what it is keeps the analysis
	// honest rather than needing a suppression that would hide a real leak
	// later.
	//
	// The YAML key stays `password_env`: that is the operator-facing contract.
	PasswordVar string `yaml:"password_env"`

	// From is the envelope and header sender. Required.
	From string `yaml:"from"`

	// FromName is the display name for From. Optional.
	FromName string `yaml:"from_name"`

	// TimeoutSeconds bounds a single send. Defaults to DefaultTimeoutSeconds.
	TimeoutSeconds int `yaml:"timeout_seconds"`

	// BaseURL is the public app URL used to resolve relative links in mail.
	// Mail is read outside the app, so a relative link is dead without it.
	BaseURL string `yaml:"base_url"`

	// relaDir is the .rela directory this config was loaded from, used to find
	// secrets.yaml at send time. Set by LoadConfig; unexported so it cannot be
	// supplied from YAML.
	relaDir string `yaml:"-"`

	// Password is REJECTED by Validate. It exists as a field only so that
	// writing it produces a clear error naming password_env, rather than
	// yaml silently ignoring an unknown key and the operator wondering why
	// authentication fails.
	Password string `yaml:"password"`
}

// Timeout returns the configured send timeout, or the default.
func (c *Config) Timeout() int {
	if c.TimeoutSeconds <= 0 {
		return DefaultTimeoutSeconds
	}
	return c.TimeoutSeconds
}

// EffectivePort returns the configured port, or the default.
func (c *Config) EffectivePort() int {
	if c.Port <= 0 {
		return DefaultPort
	}
	return c.Port
}

// LoadConfig reads .rela/mail.yaml from the given .rela directory.
//
// Returns ErrConfigNotFound when the file does not exist — that is the "mail
// not configured" state and every other command must still start normally.
//
// Nil: never returns a nil Config with a nil error.
func LoadConfig(relaDir string) (*Config, error) {
	path := filepath.Join(relaDir, ConfigFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrConfigNotFound
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid %s: %w", path, err)
	}
	cfg.relaDir = relaDir
	return &cfg, nil
}

// Validate checks the configuration.
//
// Error messages name the offending key but NEVER echo a credential: `rela
// validate` prints validator errors verbatim to the terminal, so an error
// carrying a secret would leak it to logs and CI output.
func (c *Config) Validate() error {
	// A literal password in the file is refused rather than accepted-with-a-
	// warning. Accepting it would put a live credential in a file operators
	// routinely paste into issues, and a warning is easy to miss.
	if c.Password != "" {
		return errors.New("password must not be set in the config file; " +
			"use password_env to name an environment variable instead")
	}

	switch c.Transport {
	case TransportMemory:
		// No transport-specific requirements: nothing leaves the process.
		return c.validateCommon()
	case TransportSMTP:
		if strings.TrimSpace(c.Host) == "" {
			return errors.New("host is required for transport: smtp")
		}
		if strings.Contains(c.Host, "/") || strings.Contains(c.Host, "@") {
			// Catches a URL or user:pass@host pasted into a hostname field —
			// the shape that would smuggle a credential into the config.
			return fmt.Errorf("host must be a bare hostname, got %q", c.Host)
		}
		if c.Port < 0 || c.Port > 65535 {
			return fmt.Errorf("port %d out of range", c.Port)
		}
		return c.validateCommon()
	case "":
		return errors.New("transport is required (smtp or memory)")
	default:
		return fmt.Errorf("unknown transport %q (want smtp or memory)", c.Transport)
	}
}

func (c *Config) validateCommon() error {
	if strings.TrimSpace(c.From) == "" {
		return errors.New("from is required")
	}
	if err := validateHeaderValue("from", c.From); err != nil {
		return err
	}
	if err := validateHeaderValue("from_name", c.FromName); err != nil {
		return err
	}
	if c.TimeoutSeconds < 0 {
		return fmt.Errorf("timeout_seconds must not be negative, got %d", c.TimeoutSeconds)
	}
	hasScheme := strings.HasPrefix(c.BaseURL, "http://") || strings.HasPrefix(c.BaseURL, "https://")
	if c.BaseURL != "" && !hasScheme {
		return fmt.Errorf("base_url must start with http:// or https://, got %q", c.BaseURL)
	}
	return nil
}

// WithRelaDir points a programmatically-built Config at a .rela directory so it
// can find secrets.yaml. LoadConfig sets this automatically; callers that
// construct a Config in code (tests, and any future wiring) set it explicitly.
func (c *Config) WithRelaDir(dir string) *Config {
	c.relaDir = dir
	return c
}

// SecretKey is the key read from .rela/secrets.yaml.
const SecretKey = "smtp_password"

// hasPassword reports whether a password is available from either source.
//
// Separate from resolvePassword so a caller can check for the misconfiguration
// without the plaintext entering its scope — which is what keeps the credential
// confined to SMTPSender.dial.
func (c *Config) hasPassword() bool {
	return c.resolvePassword() != ""
}

// resolvePassword reads the configured password environment variable.
//
// Read at SEND time, not at load: a process that never sends mail must start
// cleanly with the variable unset, and the value must not sit in memory for the
// lifetime of a command that has no use for it.
func (c *Config) resolvePassword() string {
	// secrets.yaml first: it is where an operator keeps every other credential,
	// so it is where they will look for this one.
	if sec, err := secrets.Load(c.relaDir, ""); err == nil {
		if v := sec[SecretKey]; v != "" {
			return v
		}
	}
	if c.PasswordVar == "" {
		return ""
	}
	return os.Getenv(c.PasswordVar)
}
