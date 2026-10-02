package dataentryconfig

import (
	"errors"
	"fmt"
	"net/url"
	"slices"
	"sort"
	"strings"
	"unicode"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// AccountConfig is the `account:` block of data-entry.yaml: the links the
// SPA's account menu offers, and where the menu finds a person's avatar
// (TKT-MJTD12). Every key is optional.
//
// rela does not own login. The proxy in front of it does, so each link names
// a PAGE the proxy owns, never one of its APIs: a sign-out that needs a CSRF
// token or a confirmation handles that on its own page, and rela never holds
// the proxy's token. The links are the same for every user of a deployment,
// which is why they are config rather than token claims.
//
// Link values are validated at load by [validAccountLink]: a root-relative
// path (one leading slash) or an absolute `https://` URL.
type AccountConfig struct {
	// SignOut is the proxy's sign-out page, e.g. `/oauth2/sign_out`.
	SignOut string `yaml:"sign_out,omitempty" json:"sign_out,omitempty"`
	// Account is the proxy's page for managing the user's own account.
	Account string `yaml:"account,omitempty" json:"account,omitempty"`
	// SwitchOrg is the proxy's org picker.
	SwitchOrg string `yaml:"switch_org,omitempty" json:"switch_org,omitempty"`

	// Admin is the proxy's administration page. Nil: accepted, means no admin
	// link.
	Admin *AccountAdminLink `yaml:"admin,omitempty" json:"admin,omitempty"`

	// AvatarProperty names the property on the person entity that holds the
	// user's photo. A `file` property is served through the attachment
	// endpoint; a `string` property holds a URL, used only when it is
	// root-relative or https. Validated at load to be a `file` or `string`
	// property declared on some entity type; the data-entry app additionally
	// checks it against acl.yaml's `user_entity_type` when that is set.
	AvatarProperty string `yaml:"avatar_property,omitempty" json:"avatar_property,omitempty"`
}

// AccountAdminLink is the admin entry of the account menu.
//
// Role is a UX filter, NOT a security boundary: it hides the link from
// principals whose asserted `roles` claim lacks it, so users are not shown a
// page they cannot use. The proxy's admin page must authorize on its own. The
// URL itself is not a secret (see "The configuration is not a secret; the
// data is" in the root CLAUDE.md), and a user who types it gets whatever the
// proxy decides.
type AccountAdminLink struct {
	// URL is the admin page; validated like the other account links.
	URL string `yaml:"url" json:"url"`
	// Role, when set, shows the link only to principals whose asserted roles
	// contain it. Empty shows it to everyone.
	Role string `yaml:"role,omitempty" json:"role,omitempty"`
}

// accountKeys and accountAdminKeys are the keys the account block accepts.
// Nested unknown keys are otherwise ignored by the decoder, so a typo such as
// `signout:` would silently drop a sign-out link; the block checks its own.
var (
	accountKeys      = []string{"account", "admin", "avatar_property", "sign_out", "switch_org"}
	accountAdminKeys = []string{"role", "url"}
)

// validateAccount checks the `account:` block: known keys only, every link a
// root-relative path or https URL, and avatar_property a declared file or
// string property. data is the raw YAML, needed for the unknown-key check.
func validateAccount(data []byte, cfg *Config, meta *metamodel.Metamodel) []string {
	errs := accountUnknownKeys(data)
	acc := cfg.Account
	if acc == nil {
		return errs
	}
	links := []struct{ key, value string }{
		{"account.sign_out", acc.SignOut},
		{"account.account", acc.Account},
		{"account.switch_org", acc.SwitchOrg},
	}
	if acc.Admin != nil {
		if acc.Admin.URL == "" {
			errs = append(errs, "account.admin.url: required when account.admin is set")
		}
		links = append(links, struct{ key, value string }{"account.admin.url", acc.Admin.URL})
	}
	for _, l := range links {
		if l.value == "" {
			continue
		}
		if err := validAccountLink(l.value); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %q %v", l.key, l.value, err))
		}
	}
	if acc.AvatarProperty != "" {
		errs = append(errs, validateAvatarProperty(acc.AvatarProperty, meta)...)
	}
	return errs
}

// accountUnknownKeys reports keys under `account:` and `account.admin:` that
// the block does not define.
func accountUnknownKeys(data []byte) []string {
	var raw struct {
		Account map[string]any `yaml:"account"`
	}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil // the struct unmarshal reports malformed YAML
	}
	errs := unknownKeysIn("account", raw.Account, accountKeys)
	if admin, ok := raw.Account["admin"].(map[string]any); ok {
		errs = append(errs, unknownKeysIn("account.admin", admin, accountAdminKeys)...)
	}
	return errs
}

func unknownKeysIn(prefix string, m map[string]any, valid []string) []string {
	var errs []string
	for key := range m {
		if !slices.Contains(valid, key) {
			errs = append(errs, fmt.Sprintf("%s: unknown key %q (valid keys: %s)",
				prefix, key, strings.Join(valid, ", ")))
		}
	}
	sort.Strings(errs)
	return errs
}

// ValidAccountLink reports whether s may be used as an account-menu link: a
// root-relative path starting with exactly one `/`, or an absolute `https://`
// URL with a host and no user info. It rejects every other scheme
// (`javascript:`, `data:`, `http:`), a protocol-relative `//host`, the
// browser-equivalent `/\host`, a relative path without a leading slash, and
// any whitespace or control character.
//
// Exported so the data-entry app applies the same rule to an avatar stored
// as a URL string.
func ValidAccountLink(s string) bool { return validAccountLink(s) == nil }

func validAccountLink(s string) error {
	for _, r := range s {
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			return errors.New("must not contain whitespace or control characters")
		}
	}
	if strings.HasPrefix(s, "/") {
		if strings.HasPrefix(s, "//") || strings.HasPrefix(s, `/\`) {
			return errors.New("must be a path on this host, not a protocol-relative URL")
		}
		return nil
	}
	u, err := url.Parse(s)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
		return errors.New("must be a path starting with / or an https:// URL")
	}
	return nil
}

// validateAvatarProperty checks that name is a `file` or `string` property on
// at least one entity type. Which type principals resolve to lives in
// acl.yaml, which this package does not load; the data-entry app checks that
// type when acl.yaml names one.
func validateAvatarProperty(name string, meta *metamodel.Metamodel) []string {
	if meta == nil {
		return nil
	}
	found := false
	for _, def := range meta.Entities {
		prop, ok := def.Properties[name]
		if !ok {
			continue
		}
		found = true
		if !AvatarPropertyType(prop.Type) {
			return []string{fmt.Sprintf(
				"account.avatar_property: property %q has type %q; must be file or string", name, prop.Type)}
		}
	}
	if !found {
		return []string{fmt.Sprintf(
			"account.avatar_property: no entity type declares a property %q", name)}
	}
	return nil
}

// AvatarPropertyType reports whether a property of type t can hold an avatar:
// a `file` attachment or a `string` URL.
func AvatarPropertyType(t string) bool {
	return t == metamodel.PropertyTypeFile || t == metamodel.PropertyTypeString
}
