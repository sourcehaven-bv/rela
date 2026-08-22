package mail

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
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
// The credential is deliberately absent. PasswordEnv names an environment
// variable read at SEND time; the password itself never appears in this file,
// never on a command line, and never in a log. That is the same invariant
// RELA_DATABASE_URL carries — a secret must not reach `ps` output or shell
// history — and Validate enforces the file half of it.
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

	// PasswordEnv names the environment variable holding the SMTP password.
	// It is NOT the password. Resolved at send time so commands that never
	// send mail start fine with the variable unset.
	PasswordEnv string `yaml:"password_env"`

	// From is the envelope and header sender. Required.
	From string `yaml:"from"`

	// FromName is the display name for From. Optional.
	FromName string `yaml:"from_name"`

	// TimeoutSeconds bounds a single send. Defaults to DefaultTimeoutSeconds.
	TimeoutSeconds int `yaml:"timeout_seconds"`

	// BaseURL is the public app URL used to resolve relative links in mail.
	// Mail is read outside the app, so a relative link is dead without it.
	BaseURL string `yaml:"base_url"`

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
		if c.Username != "" && c.PasswordEnv == "" {
			return errors.New("password_env is required when username is set " +
				"(omit username for a relay that accepts unauthenticated submission)")
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

// hasPassword reports whether the configured password environment variable
// holds a value.
//
// Separate from resolvePassword so a caller can check for the misconfiguration
// without the plaintext entering its scope — which is what keeps the credential
// confined to SMTPSender.dial.
func (c *Config) hasPassword() bool {
	return c.PasswordEnv != "" && os.Getenv(c.PasswordEnv) != ""
}

// resolvePassword reads the configured password environment variable.
//
// Read at SEND time, not at load: a process that never sends mail must start
// cleanly with the variable unset, and the value must not sit in memory for the
// lifetime of a command that has no use for it.
func (c *Config) resolvePassword() string {
	if c.PasswordEnv == "" {
		return ""
	}
	return os.Getenv(c.PasswordEnv)
}
