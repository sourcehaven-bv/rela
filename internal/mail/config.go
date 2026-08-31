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

	// Recipients declares who mail.send may address (TKT-USQNA3).
	//
	// A POINTER, and that is the whole design. Absent (`nil`) must be
	// distinguishable from present-but-empty, because the two get different
	// errors: nil says "add a recipients: block", empty-but-valid says "this
	// address is not in the set you declared". A value type collapses both
	// into the zero struct and the operator gets sent to the wrong place.
	//
	// Absent DENIES EVERYTHING. That inverts this file's usual rule — an
	// absent mail.yaml means "mail is off", an absent port means 587 — and
	// the inversion is deliberate. Permitting on absence fails silently and
	// irreversibly (mail leaves the ACL perimeter and nobody knows until the
	// recipient replies); refusing on absence fails loudly and harmlessly (a
	// typed error naming this key, and four lines of YAML to fix it). A
	// control whose unconfigured state is "allow" is not a control.
	Recipients *RecipientConfig `yaml:"recipients"`

	// Password is REJECTED by Validate. It exists as a field only so that
	// writing it produces a clear error naming password_env, rather than
	// yaml silently ignoring an unknown key and the operator wondering why
	// authentication fails.
	Password string `yaml:"password"`
}

// RecipientConfig is the `recipients:` block of .rela/mail.yaml.
//
// This package PARSES and VALIDATES it; it resolves nothing. Resolution needs
// store, filter and metamodel, and .go-arch-lint.yml withholds all three from
// internal/mail precisely so a send script cannot reach the graph. So the
// enforcement lives in internal/lua — which already holds the reader seam —
// and this type's only job is to be the operator-facing shape and to convert
// itself into a [lua.RecipientPolicy] the moment it is loaded.
type RecipientConfig struct {
	// Query selects recipient entities from the graph, in the form
	// `TYPE [where EXPR [and EXPR]...]` — e.g. `person where status =
	// 'active'`. The type alone is legal and means every entity of that type.
	//
	// This is the PRIMARY mechanism, because a recipient is normally an
	// entity: an allowlist derived from the graph tracks reality as people
	// join and leave, where a hand-maintained address list drifts the moment
	// somebody forgets to edit it.
	Query string `yaml:"query"`

	// Property names the entity property holding the address, e.g. `email`.
	// Required whenever Query is set — an entity set with no address property
	// names nobody.
	Property string `yaml:"property"`

	// AlsoAllow holds literal addresses that are NOT entities: an ops alias,
	// an external auditor's mailbox, a monitoring endpoint. Unioned with the
	// query result.
	//
	// LITERAL ONLY. A wildcard such as `*@example.com` is REFUSED at load —
	// see validateAlsoAllow for why refusing beats both ignoring it and
	// implementing it.
	AlsoAllow []string `yaml:"also_allow"`

	// AllowAny disables the check entirely: every address is permitted.
	//
	// The escape hatch for a deployment that has decided this constraint is
	// not for them. It must be a deliberate `allow_any: true` line in the
	// file — it is never a default, never inferred from an empty block, and
	// never reached by omission, so it stays greppable in a config review.
	AllowAny bool `yaml:"allow_any"`
}

// Policy converts the operator's block into the runtime's gate input.
//
// Address normalization happens HERE, once at load, through
// [lua.NormalizeRecipient] rather than a local copy: both sides must fold
// addresses by the identical rule, and two independent normalizations that
// drift is how an allowlist starts admitting what it should not.
func (r *RecipientConfig) Policy() (lua.RecipientPolicy, error) {
	if r == nil {
		// The zero policy, which denies. Reachable only from a Config whose
		// block is absent; the caller relies on this rather than checking nil
		// itself, so "absent" has one meaning in one place.
		return lua.RecipientPolicy{}, nil
	}
	policy := lua.RecipientPolicy{Configured: true, AllowAny: r.AllowAny, Property: r.Property}
	for _, addr := range r.AlsoAllow {
		policy.AlsoAllow = append(policy.AlsoAllow, lua.NormalizeRecipient(addr))
	}
	entityType, filters, err := parseRecipientQuery(r.Query)
	if err != nil {
		return lua.RecipientPolicy{}, err
	}
	policy.EntityType = entityType
	policy.Filters = filters
	return policy, nil
}

// validateRecipients checks the `recipients:` block.
//
// Called from validateCommon so it applies to every transport: which server
// carries the mail has no bearing on who may receive it.
func (c *Config) validateRecipients() error {
	r := c.Recipients
	if r == nil {
		// Absent is VALID configuration that DENIES at send time. Refusing to
		// load would be wrong: a project with no mail.yaml at all already
		// starts fine, and a project that configures a transport but has not
		// yet decided its recipients must still run every other command. The
		// refusal belongs at the send, where it can name the key.
		return nil
	}
	if strings.TrimSpace(r.Query) == "" && len(r.AlsoAllow) == 0 && !r.AllowAny {
		// A block that permits nothing is refused rather than accepted as a
		// deny-everything policy. It LOOKS configured, so the operator would
		// read the resulting denials as a bug in rela rather than as their own
		// empty block — and "deny everything" is already available by deleting
		// the block, which at least produces an error that says so.
		return errors.New("recipients must set at least one of query, also_allow or allow_any " +
			"(an empty block permits nobody; omit it entirely to deny by default)")
	}
	if strings.TrimSpace(r.Query) != "" && strings.TrimSpace(r.Property) == "" {
		return errors.New("recipients.property is required when recipients.query is set " +
			"(it names the entity property holding the address, e.g. `property: email`)")
	}
	if _, _, err := parseRecipientQuery(r.Query); err != nil {
		// Parsed at LOAD as well as at send, so a typo in the query surfaces
		// from `rela validate` rather than from the first message that fails
		// to go out.
		return fmt.Errorf("recipients.query: %w", err)
	}
	return validateAlsoAllow(r.AlsoAllow)
}

// validateAlsoAllow checks the literal-address list.
//
// # Wildcards are refused, deliberately and with the syntax reserved
//
// `*@example.com` does not work, and a `*` anywhere in an entry is an error
// rather than an ordinary character.
//
// Refusing beats ignoring. A literal-only matcher that silently treated `*` as
// a normal character would accept the config, match nothing, and present as
// "mail is mysteriously denied" — the operator believing they allowed a domain
// while the behaviour is that they allowed an address that cannot exist.
//
// Refusing also RESERVES the syntax, which is the real reason to decide this
// now rather than later. If `*` meant "literal asterisk" in a shipped config
// key, making it a metacharacter afterwards would break every deployment that
// had (however improbably) relied on it. With `*` refused today, adding domain
// wildcards later can only ever turn an error into a success — compatible by
// construction.
//
// Why not simply implement them now: a domain wildcard is a strictly weaker
// control than a query. `*@example.com` permits every address at the domain,
// including ones belonging to nobody in the graph and ones that never existed,
// while `query` tracks who actually exists. Shipping the weaker control
// alongside the stronger one invites it to become the default. It is one
// function away when a deployment genuinely needs it.
func validateAlsoAllow(addrs []string) error {
	for i, addr := range addrs {
		if strings.TrimSpace(addr) == "" {
			return fmt.Errorf("recipients.also_allow[%d] is empty", i)
		}
		if strings.Contains(addr, "*") {
			return fmt.Errorf("recipients.also_allow[%d] %q contains a wildcard; "+
				"only literal addresses are supported — use recipients.query to select "+
				"recipients from the graph instead", i, addr)
		}
		// also_allow entries reach an SMTP envelope, so they get the same
		// header-injection check every other caller-supplied address gets.
		if err := validateHeaderValue(fmt.Sprintf("recipients.also_allow[%d]", i), addr); err != nil {
			return err
		}
	}
	return nil
}

// recipientQueryKeyword separates the entity type from its conditions.
const recipientQueryKeyword = " where "

// recipientFilterSeparator joins conditions inside a query.
const recipientFilterSeparator = " and "

// parseRecipientQuery lowers `TYPE [where EXPR [and EXPR]...]` onto the pieces
// that already exist: an entity type plus [filter] expressions.
//
// The ticket specifies the SQL-ish surface, but rela has no such parser and
// inventing a second query dialect for one config key would mean an operator
// learning two ways to say `status=active`. So this is a lowering, not a
// language: the leading word is the type, the rest is split on ` and ` and
// handed to filter.Parse. Every semantic question — globs, ranges, regex,
// what `=` means for a date — is therefore answered identically here and in
// schedules.yaml's for_each.
//
// Quotes around a value are stripped because the ticket's own example writes
// `status = 'active'` and filter.Parse would otherwise match the literal
// four-character value `active` including its quotes — a config that looks
// right, parses, and matches nothing.
//
// An empty query is not an error: it means no query was configured, which is
// legal as long as also_allow or allow_any carries the policy. Validate
// enforces that.
func parseRecipientQuery(q string) (entityType string, filters []*filter.Filter, err error) {
	q = strings.TrimSpace(q)
	if q == "" {
		return "", nil, nil
	}
	head, tail, hasWhere := strings.Cut(strings.ToLower(q), strings.TrimSpace(recipientQueryKeyword))
	// Cut on the lowercased copy to accept `WHERE`, but slice the ORIGINAL so
	// a property name or value keeps its case — entity types and property
	// values are case-sensitive and folding them here would break the match.
	if hasWhere {
		head, tail = q[:len(head)], q[len(q)-len(tail):]
	} else {
		head, tail = q, ""
	}
	entityType = strings.TrimSpace(head)
	if entityType == "" {
		return "", nil, fmt.Errorf("query %q does not name an entity type", q)
	}
	if strings.ContainsAny(entityType, " \t") {
		// Catches `person status = 'active'` — a missing `where` that would
		// otherwise become an entity type nothing can match.
		return "", nil, fmt.Errorf("query %q: expected `TYPE` or `TYPE where CONDITION`", q)
	}
	if strings.TrimSpace(tail) == "" {
		if hasWhere {
			return "", nil, fmt.Errorf("query %q has a `where` with no condition after it", q)
		}
		return entityType, nil, nil
	}
	for _, expr := range strings.Split(tail, recipientFilterSeparator) {
		f, parseErr := filter.Parse(stripQuotes(expr))
		if parseErr != nil {
			return "", nil, fmt.Errorf("query %q: %w", q, parseErr)
		}
		filters = append(filters, f)
	}
	return entityType, filters, nil
}

// stripQuotes removes matching single or double quotes from a filter
// expression's VALUE side, so `status = 'active'` compares against `active`.
//
// The value side only: a quote in a property name is a typo, and stripping
// quotes blindly across the whole expression would silently accept it.
func stripQuotes(expr string) string {
	expr = strings.TrimSpace(expr)
	for _, op := range []string{"!=", ">=", "<=", "=~", "=", ">", "<", "~"} {
		lhs, rhs, found := strings.Cut(expr, op)
		if !found {
			continue
		}
		rhs = strings.TrimSpace(rhs)
		if len(rhs) >= 2 && (rhs[0] == '\'' || rhs[0] == '"') && rhs[len(rhs)-1] == rhs[0] {
			rhs = rhs[1 : len(rhs)-1]
		}
		return strings.TrimSpace(lhs) + op + rhs
	}
	return expr
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
func (c ScriptCapabilities) toLua() lua.Capabilities {
	return lua.Capabilities{
		HTTP:      c.HTTP,
		AI:        false,
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
