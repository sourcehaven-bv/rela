package metamodel

import "fmt"

// Capabilities is the operator-authored declaration of which ambient, non-graph
// capabilities a Lua script may reach (TKT-YH52OM). It is the YAML face of
// lua.Capabilities; the wiring site translates one to the other.
//
// It appears under a `capabilities:` key on the config blocks that name a
// script — automation actions, data-entry actions, and documents:
//
//	actions:
//	  notify_slack:
//	    script: notify.lua
//	    capabilities:
//	      http: true
//	      secrets: [slack_webhook_url]
//
// # Fail-closed
//
// Omitting the block grants nothing. That is deliberate and is the whole point
// of the ticket: before this existed, every script on every surface — including
// read-only document renders and validation rules — held `http`, `ai` and the
// ENTIRE contents of .rela/secrets.yaml, which is a two-call exfiltration path.
// A default of "closed" means a capability is present only where an operator
// wrote it down.
//
// # Secrets is a list
//
// [Capabilities.Secrets] names individual keys rather than being a boolean, because a
// boolean grants the whole file: an action needing one Slack webhook would also
// receive the database DSN. There is deliberately no "all" spelling here — the
// broad grant exists only as a Go-side wiring choice
// (lua.TrustedCapabilities), so it cannot be reached from a config file.
type Capabilities struct {
	// HTTP grants the `http` global (outbound requests).
	HTTP bool `yaml:"http,omitempty"`

	// AI grants the `ai` global. Note ai.* calls are billable.
	AI bool `yaml:"ai,omitempty"`

	// Mail grants mail.send (TKT-JVHSOZ). Without it the binding still
	// exists — so a script can feature-detect — but refuses with
	// `err.kind == "denied"`. See the Mail field on lua.Capabilities for why
	// this one gate does not work by withholding the binding.
	Mail bool `yaml:"mail,omitempty"`

	// WriteFile grants rela.write_file (already confined to output/).
	WriteFile bool `yaml:"write_file,omitempty"`

	// Secrets names the keys from .rela/secrets.yaml this script may read.
	// A key not listed is absent from rela.secrets entirely.
	Secrets []string `yaml:"secrets,omitempty"`
}

// Any reports whether the block grants anything at all.
//
// Note this deliberately has no AllSecrets term, unlike lua.Capabilities.Any:
// there is no "all secrets" spelling in YAML, so a config block can only grant
// via the named list. See the AllSecrets field on lua.Capabilities.
func (c Capabilities) Any() bool {
	return c.HTTP || c.AI || c.Mail || c.WriteFile || len(c.Secrets) > 0
}

// Fields returns the grant as plain values.
//
// This is the SINGLE translation seam between the YAML type and every runtime
// consumer (TKT-YH52OM). It exists because the obvious alternative — each
// consumer copying the struct field-by-field — is what produced the defect this
// method was added to prevent: the grant was hand-copied at five sites, and a
// sixth path dropped it silently.
//
// It returns loose values rather than a lua.Capabilities because metamodel must
// not import lua, and autocascade may import neither (see .go-arch-lint.yml).
// Each consumer converts at its own boundary, but they all read the fields from
// here, so adding a capability means changing this signature — a COMPILE error
// at every consumer rather than a silent per-surface omission.
func (c Capabilities) Fields() (http, ai, mail, writeFile bool, secrets []string) {
	return c.HTTP, c.AI, c.Mail, c.WriteFile, c.Secrets
}

// UnmarshalYAML decodes the mapping form and REFUSES a bare boolean.
//
// `capabilities: true` is rejected rather than read as "grant everything",
// following the same reasoning [ACLBypass.UnmarshalYAML] records for
// allow_acl_bypass: for a privilege field, a parser that maps a loose value to
// the BROADEST setting is the wrong default. Here it is sharper still, since
// "everything" would include the entire secrets file — precisely the grant this
// type exists to make impossible to write by accident.
func (c *Capabilities) UnmarshalYAML(unmarshal func(any) error) error {
	// A YAML bool decodes cleanly into a string, so catch it by value before
	// attempting the mapping decode (same trick ACLBypass uses).
	var raw string
	if err := unmarshal(&raw); err == nil {
		return fmt.Errorf(
			"capabilities must be a mapping, not %q: write e.g. "+
				"`capabilities: {http: true, secrets: [my_key]}`. There is no "+
				"grant-everything form — name the secrets the script needs", raw)
	}

	// alias avoids recursing into this method.
	type plain Capabilities
	var p plain
	if err := unmarshal(&p); err != nil {
		return err
	}
	*c = Capabilities(p)
	return nil
}
