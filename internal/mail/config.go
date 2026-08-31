package mail

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/lua"
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

	// TransportHTTP delivers through the SimpleMailService APIv2 HTTP API.
	// See http.go, including why exactly one provider is compiled in.
	TransportHTTP Transport = "http"

	// TransportScript delivers by running an operator Lua script. The general
	// answer for any provider rela does not ship — see script.go.
	TransportScript Transport = "script"
)

// transportList is the closed set, for error messages. One source so a new
// transport cannot be added to the switch and forgotten in the message an
// operator actually reads.
const transportList = "smtp, memory, http or script"

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
//  1. `.rela/secrets.yaml` — under `smtp_password` for the SMTP transport and
//     `mail_api_token` for the HTTP one. The same store Lua scripts read, and
//     the natural home for both: neither is different in kind from the API
//     tokens already kept there.
//  2. The environment variable named by `password_env`, if set.
//
// Either way the credential never appears in mail.yaml, never on a command
// line, and never in a log — the invariant RELA_DATABASE_URL carries, because
// a secret must not reach `ps` output or shell history.
//
// secrets.yaml first, because that is where an operator will look. password_env
// stays for deployments that inject credentials as environment variables
// (containers, systemd units) and would otherwise have to materialize a
// plaintext file at deploy time.
//
// `transport: script` is the exception: its credential is whatever key the
// script names, granted through `capabilities.secrets` and read as
// `rela.secrets.<key>`. Nothing here resolves it, because only the script
// knows what its provider wants.
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

	// AccountID is the provider account the APIv2 endpoint is scoped to.
	// Required for TransportHTTP; it is a path segment, not a credential.
	AccountID string `yaml:"account_id"`

	// Script is the project-relative path to the Lua send script. Required
	// for TransportScript.
	//
	// Project-relative, and refused if it escapes the project (see
	// scriptPathIsContained): the script runs with outbound HTTP and a named
	// credential, so `../../../tmp/x.lua` is worth refusing at load rather
	// than discovering later.
	Script string `yaml:"script"`

	// Capabilities is the ambient grant the send script runs with. Only
	// meaningful for TransportScript.
	//
	// Fail-closed (TKT-YH52OM): omitted means the script gets NOTHING — no
	// http, no secrets — so a typo'd key name produces a script that cannot
	// reach the network rather than one that quietly can.
	Capabilities ScriptCapabilities `yaml:"capabilities"`

	// Password is REJECTED by Validate. It exists as a field only so that
	// writing it produces a clear error naming password_env, rather than
	// yaml silently ignoring an unknown key and the operator wondering why
	// authentication fails.
	Password string `yaml:"password"`
}

// ScriptCapabilities is the YAML face of [lua.Capabilities] for a send script.
//
// A separate type rather than embedding lua.Capabilities directly, for two
// reasons. It keeps the YAML surface NARROWER than the Go one — `write_file`
// and `ai` are absent here because a mail transport has no business writing
// files or spending money on inference, and a field that exists in YAML is a
// field an operator will eventually set. And lua.Capabilities carries
// AllSecrets, which is deliberately not settable from config; re-exposing the
// struct wholesale would be one embedded field away from an operator writing
// `all_secrets: true` in mail.yaml.
type ScriptCapabilities struct {
	// HTTP registers the `http` global. Effectively required for any real
	// provider, but not defaulted on: "this script may reach the network" is
	// a sentence the operator should have written.
	HTTP bool `yaml:"http"`

	// Secrets names the keys from .rela/secrets.yaml the script may read.
	//
	// A LIST of key names, never a bool. A boolean would hand a send script
	// the database DSN and every other API key along with the one credential
	// it needs — see the lua.Capabilities doc for the full argument.
	Secrets []string `yaml:"secrets"`
}

// toLua converts the config grant into the runtime's capability type.
//
// AI and WriteFile are hard-wired false, not merely unset: a send script has
// no legitimate use for either, and a zero value that happened to change
// meaning later should not silently grant them here.
//
// Mail is hard-wired TRUE, and it is the one grant in the codebase that is not
// the operator's to withhold. mail.send is capability-gated everywhere else
// (TKT-JVHSOZ), but THIS runtime is the implementation of mail.send: it is the
// script `transport: script` invokes to actually deliver a message. Gating it
// would be circular — the runtime whose entire job is sending mail would need
// permission from the subsystem it IS in order to do that job, and an operator
// who forgot the key would get a mail system that silently refuses to mail.
//
// The gate loses nothing by conceding this. Its purpose is to stop an ARBITRARY
// script from reaching an outbound channel, and this is not an arbitrary
// script: mail.yaml names it, mail.yaml lives in the same audited operator tier
// as acl.yaml, and configuring a send script is already the act of saying "this
// code sends mail". There is deliberately no `mail:` key on ScriptCapabilities
// for an operator to set — a knob whose only correct position is "on" is a
// knob that can be turned off by mistake.
//
// Note this does NOT hand the send script an unbounded outbound channel by the
// back door: `mail` authorizes the mail.send binding, and reaching a recipient
// still means going through the transport the operator configured.
//
// Note also what this field does NOT unblock today: buildRuntime passes no
// WithMailSender, so a send script calling mail.send still gets
// `not_configured` — recursing into the mail system from inside it is not
// something the design intends, and that is pinned by
// TestScriptSender_InnerRuntimeHasNoMailSender.
//
// The distinction is worth keeping straight, because it is what the grant buys:
// `not_configured` means the call was AUTHORIZED and found no transport, which
// is the honest answer. Drop this field and the same call reports `denied` —
// the mail subsystem refusing itself permission to mail, in the one place an
// operator would never think to look for a capability problem.
func (c ScriptCapabilities) toLua() lua.Capabilities {
	return lua.Capabilities{
		HTTP:      c.HTTP,
		AI:        false,
		Mail:      true,
		WriteFile: false,
		Secrets:   c.Secrets,
	}
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
	case TransportHTTP:
		if strings.TrimSpace(c.AccountID) == "" {
			return errors.New("account_id is required for transport: http")
		}
		// The account ID becomes a URL path segment, so a value containing a
		// slash or a traversal component would silently retarget the request
		// at a different endpoint on the same host.
		if strings.ContainsAny(c.AccountID, "/?#") || strings.Contains(c.AccountID, "..") {
			return fmt.Errorf("account_id must be a plain identifier, got %q", c.AccountID)
		}
		return c.validateCommon()
	case TransportScript:
		if strings.TrimSpace(c.Script) == "" {
			return errors.New("script is required for transport: script")
		}
		if !scriptPathIsContained(c.Script) {
			return fmt.Errorf("script must be a project-relative path inside the project, got %q", c.Script)
		}
		if !strings.HasSuffix(c.Script, ".lua") {
			return fmt.Errorf("script must be a .lua file, got %q", c.Script)
		}
		return c.validateCommon()
	case "":
		return fmt.Errorf("transport is required (%s)", transportList)
	default:
		return fmt.Errorf("unknown transport %q (want %s)", c.Transport, transportList)
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
	sec, err := secrets.Load(c.relaDir, "")
	switch {
	case errors.Is(err, secrets.ErrNotFound):
		// No secrets configured at all — the ordinary case for a deployment
		// using password_env. Fall through silently.
	case err != nil:
		// A malformed or unreadable secrets source is NOT the same as an
		// absent one. Swallowing it here would send the caller on to
		// password_env and, when that is unset, produce "password_env is
		// empty or unset" — an error naming the wrong cause entirely. The
		// send still falls back, because refusing to send mail over a
		// secrets-file syntax error is worse than trying, but the real fault
		// is on the record. The error names the path, never the contents.
		slog.Warn("mail: secrets source unreadable, falling back to password_env", "error", err)
	default:
		if v := sec[SecretKey]; v != "" {
			return v
		}
	}
	if c.PasswordVar == "" {
		return ""
	}
	return os.Getenv(c.PasswordVar)
}

// resolveAPIToken reads the HTTP transport's bearer token.
//
// Same two sources and same order as resolvePassword — secrets.yaml
// first, then the environment variable named by password_env — under a
// different key, because a bearer token and an SMTP password are different
// credentials and a project that migrates from one transport to the other
// should not silently authenticate with the wrong one.
//
// Read at SEND time for the same reason: a process that never sends mail must
// start cleanly with nothing configured, and the value must not sit in memory
// for the lifetime of a command that has no use for it.
func (c *Config) resolveAPIToken() string {
	if sec, err := secrets.Load(c.relaDir, ""); err == nil {
		if v := sec[HTTPSenderSecretKey]; v != "" {
			return v
		}
	}
	if c.PasswordVar == "" {
		return ""
	}
	return os.Getenv(c.PasswordVar)
}
