package metamodel

import (
	"fmt"
	"strings"
)

// ACLBypass declares which ACL-bypassing capabilities a Lua surface may
// unlock through rela.bypass_acl (TKT-D8T148, TKT-Y3JVFK).
//
// It is a ROUGH GUARD, not a permission model. Its job is to tell whoever
// deploys a script whether its bypass block needs reading carefully:
//
//   - absent  ⇒ the script can only do what the invoking principal can. The
//     ACL already bounds it, so it needs no special scrutiny.
//   - present ⇒ a human reads that closure before deployment.
//
// The value says WHICH KIND of scrutiny — is this reading data the principal
// cannot see, or writing past their permissions? That is all the review
// decision needs, which is why the capability is not sliced by verb
// (`create`, `update`, `delete`). Verb granularity would add config surface
// without changing what the reviewer does. It would also name capabilities
// the elevated handle does not have: there is no elevated create_entity or
// update_entity today, and config naming a nonexistent capability is worse
// than a missing field because it appears to work (DEC-O59WM4).
//
// If elevated entity creation ever lands, a set-valued form
// (`allow_acl_bypass: [read, write]`) would let verbs be added without a
// second migration. Do not grow this into a string-matched grammar.
type ACLBypass string

const (
	// ACLBypassNone is the zero value: no elevation. rela.bypass_acl is not
	// registered at all, so the script cannot elevate however it is written.
	ACLBypassNone ACLBypass = ""
	// ACLBypassRead unlocks the elevated READ methods only
	// (admin.get_entity / list_entities / get_relations). The admin table has
	// no write methods, so the surface is structurally unable to mutate. This
	// is what a document render uses to aggregate over rows its caller cannot
	// see.
	ACLBypassRead ACLBypass = "read"
	// ACLBypassWrite unlocks the elevated WRITE methods only
	// (admin.create_relation / delete_relation / delete_entity).
	ACLBypassWrite ACLBypass = "write"
	// ACLBypassReadWrite unlocks both. This is what `allow_acl_bypass: true`
	// meant before the enum existed, and what the migration rewrites it to.
	ACLBypassReadWrite ACLBypass = "read+write"
)

// AllowsRead reports whether elevated reads are unlocked.
func (a ACLBypass) AllowsRead() bool {
	return a == ACLBypassRead || a == ACLBypassReadWrite
}

// AllowsWrite reports whether elevated writes are unlocked.
func (a ACLBypass) AllowsWrite() bool {
	return a == ACLBypassWrite || a == ACLBypassReadWrite
}

// Enabled reports whether any elevation is unlocked.
func (a ACLBypass) Enabled() bool { return a != ACLBypassNone }

// UnmarshalYAML accepts only the string forms. The legacy boolean
// `allow_acl_bypass: true` is REFUSED with a message naming the replacement,
// rather than silently reinterpreted.
//
// Accepting the bool would be the easy path and is deliberately not taken.
// This field grants ACL bypass, and a parser that maps a legacy value to the
// BROADEST setting is the wrong default for a privilege field — if the two
// spellings ever drift, the shim resolves toward more access. A compatibility
// shim also has no forcing function that ever removes it, so two
// representations of one concept would persist indefinitely and every reader
// (validation, docs, tooling, the next capability added) would handle both.
// `rela migrate` rewrites `true` ⇒ `read+write`, which is the one-time cost
// that buys a single representation.
func (a *ACLBypass) UnmarshalYAML(unmarshal func(any) error) error {
	var raw string
	if err := unmarshal(&raw); err != nil {
		return err
	}
	norm := strings.ToLower(strings.TrimSpace(raw))

	// A YAML bool decodes cleanly INTO a string ("true"), so it never reaches
	// a type error — it has to be caught by value. Every spelling YAML 1.1
	// treats as a bool is listed, so `allow_acl_bypass: yes` gets the
	// migration message too rather than the generic "invalid value".
	switch norm {
	case "true", "yes", "on", "y", "false", "no", "off", "n":
		return fmt.Errorf(
			"allow_acl_bypass no longer accepts a boolean (got %q): write %q (what the "+
				"old `true` meant), %q, or %q. Run `rela migrate` to rewrite existing files",
			raw, ACLBypassReadWrite, ACLBypassRead, ACLBypassWrite)
	}

	switch v := ACLBypass(norm); v {
	case ACLBypassNone, ACLBypassRead, ACLBypassWrite, ACLBypassReadWrite:
		*a = v
		return nil
	default:
		return fmt.Errorf("invalid allow_acl_bypass value %q (want %q, %q or %q)",
			raw, ACLBypassRead, ACLBypassWrite, ACLBypassReadWrite)
	}
}
