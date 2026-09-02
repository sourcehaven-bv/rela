package dataentryconfig

import (
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// webhookIDRegex constrains a hook id. The id IS the URL path segment
// (/hooks/<id>), so it is restricted to characters that need no escaping and
// cannot introduce a path separator or traversal.
var webhookIDRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)

// webhookHeaderRegex constrains an allowlisted header name to the RFC 7230
// token grammar, so a config entry cannot smuggle a separator or whitespace.
var webhookHeaderRegex = regexp.MustCompile(`^[A-Za-z0-9!#$%&'*+._` + "`" + `|~^-]{1,128}$`)

// forbiddenWebhookHeaders are never exposable to templates, whatever the
// operator writes. Each carries caller credentials or a proxy-asserted identity,
// and interpolating one into stored entity content would persist a secret into
// the graph where it is served back out on every read.
//
// This is a floor UNDER the allowlist, not a substitute for it: the allowlist
// is what makes exposure opt-in, and this stops the one class of opt-in that is
// always a mistake. Compared lowercase.
var forbiddenWebhookHeaders = map[string]bool{
	"authorization":       true,
	"proxy-authorization": true,
	"cookie":              true,
	"set-cookie":          true,
	// Proxy-asserted identity: rela's own principal header and the common
	// oauth2-proxy/forward-auth family. Exposing these would let a hook write
	// an authenticated identity into entity content as if it were payload.
	"x-forwarded-user":               true,
	"x-forwarded-email":              true,
	"x-forwarded-preferred-username": true,
	"x-forwarded-access-token":       true,
	"x-auth-request-user":            true,
	"x-auth-request-email":           true,
	"x-auth-request-access-token":    true,
	"x-remote-user":                  true,
	// Other identity-proxy families seen in the wild.
	"x-authentik-username": true,
	"x-authentik-email":    true,
	"x-authentik-groups":   true,
	"x-vouch-user":         true,
	"x-vouch-idp-claims":   true,
}

// forbiddenWebhookHeaderPrefixes refuses whole identity-proxy families rather
// than named members.
//
// The exact-name list above was already incomplete once — it covered
// `x-forwarded-user` because that is the DOCUMENTED example, while every other
// proxy's spelling went unprotected. A prefix rule fails safe for the next
// proxy nobody has heard of yet, at the cost of refusing an occasional benign
// header an operator would have to rename. That trade is right: the failure
// mode it prevents is persisting an authenticated identity into entity content
// as if it were payload data.
var forbiddenWebhookHeaderPrefixes = []string{
	"x-forwarded-",
	"x-auth-request-",
	"x-remote-",
	"x-authentik-",
	"x-pomerium-",
}

// extraForbiddenWebhookHeaders holds names registered at wiring time, for
// header names that are only knowable at startup.
//
// The deployment's own principal header is the motivating case: it is set by
// -principal-header (or $RELA_PRINCIPAL_HEADER) to an ARBITRARY name, so the
// static list above cannot name it. It is also the single header most certain
// to carry an authenticated identity in that deployment — precisely the one the
// floor must cover. Registration is process-global because config load has no
// handle on server flags; that is acceptable for a value fixed at startup.
var extraForbiddenWebhookHeaders = map[string]bool{}

// ForbidWebhookHeader registers a header name that webhook configs may not
// expose, in addition to the built-in list. Call it at wiring time, before
// config load. Case-insensitive; an empty name is ignored.
func ForbidWebhookHeader(name string) {
	if name = strings.ToLower(strings.TrimSpace(name)); name != "" {
		extraForbiddenWebhookHeaders[name] = true
	}
}

// isForbiddenWebhookHeader reports whether a header name may not be exposed to
// a hook's templates. Name is compared lowercase against the exact-name list,
// the wiring-time registrations, and the family prefixes.
func isForbiddenWebhookHeader(name string) bool {
	lower := strings.ToLower(name)
	if forbiddenWebhookHeaders[lower] || extraForbiddenWebhookHeaders[lower] {
		return true
	}
	for _, prefix := range forbiddenWebhookHeaderPrefixes {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return false
}

// maxWebhookBodyCap is the ceiling an operator may set for max_body_bytes. A
// parsed body becomes an in-memory structure, so the cap bounds real memory per
// concurrent delivery; letting config raise it without limit would defeat it.
const maxWebhookBodyCap = 8 << 20 // 8 MiB

// validateWebhooks validates the webhooks: map. Every problem here is a LOAD
// ERROR, matching how an automation condition: that fails to compile is treated:
// a webhook that silently does not fire, or fires against the wrong type, loses
// data from a producer that generally does not retry. Failing the load is the
// safe direction.
func validateWebhooks(cfg *Config, meta *metamodel.Metamodel) []string {
	var errs []string

	for _, id := range sortedWebhookIDs(cfg) {
		hook := cfg.Webhooks[id]
		if !webhookIDRegex.MatchString(id) {
			errs = append(errs, fmt.Sprintf(
				"webhooks: invalid hook ID %q (must match ^[a-z0-9][a-z0-9_-]{0,63}$; the ID is the URL segment)", id))
			continue
		}
		errs = append(errs, validateWebhookShape(id, hook, meta)...)
		errs = append(errs, validateWebhookHeaders(id, hook)...)
		errs = append(errs, validateWebhookRespond(id, hook)...)
		errs = append(errs, validateWebhookSteps(id, hook, meta)...)
	}
	return errs
}

// validateWebhookShape checks the find/create combination and the entity types
// and properties they name.
func validateWebhookShape(id string, hook Webhook, meta *metamodel.Metamodel) []string {
	var errs []string

	if hook.Find == nil && hook.CreateIfMissing == nil {
		return append(errs, fmt.Sprintf(
			"webhooks: %q has neither find nor create_if_missing (nothing to do)", id))
	}

	if hook.Find != nil {
		errs = append(errs, validateWebhookFind(id, hook, meta)...)
	}

	if hook.CreateIfMissing != nil {
		createType := hook.CreateType()
		if createType == "" {
			errs = append(errs, fmt.Sprintf(
				"webhooks: %q create_if_missing needs a type (set create_if_missing.type or find.type)", id))
		} else if def, ok := metaEntityDef(meta, createType); !ok {
			errs = append(errs, fmt.Sprintf(
				"webhooks: %q create_if_missing type %q is not a known entity type", id, createType))
		} else {
			for _, prop := range sortedKeysOfStringMap(hook.CreateIfMissing.Properties) {
				if _, known := def.Properties[prop]; !known {
					errs = append(errs, fmt.Sprintf(
						"webhooks: %q create_if_missing sets unknown property %q on type %q", id, prop, createType))
				}
			}
		}
	}
	return errs
}

// validateWebhookFind checks find.type, find.match and find.values.
func validateWebhookFind(id string, hook Webhook, meta *metamodel.Metamodel) []string {
	var errs []string
	find := hook.Find

	if find.Type == "" {
		return append(errs, fmt.Sprintf("webhooks: %q find requires a type", id))
	}
	def, ok := metaEntityDef(meta, find.Type)
	if !ok {
		return append(errs, fmt.Sprintf(
			"webhooks: %q find type %q is not a known entity type", id, find.Type))
	}

	// A match key is required only when a create can happen. Without one,
	// find-or-create would create on every delivery: the find could not
	// identify the entity the previous delivery made, so each alert would mint
	// a duplicate rather than accumulate — silent, and only visible as growth.
	if len(find.Match) == 0 && hook.CreateIfMissing != nil {
		errs = append(errs, fmt.Sprintf(
			"webhooks: %q has create_if_missing but find declares no match properties "+
				"(a match key is required to avoid creating a duplicate per delivery)", id))
	}

	for _, prop := range find.Match {
		if _, known := def.Properties[prop]; !known {
			errs = append(errs, fmt.Sprintf(
				"webhooks: %q find matches on unknown property %q of type %q", id, prop, find.Type))
		}
	}

	matched := make(map[string]bool, len(find.Match))
	for _, prop := range find.Match {
		matched[prop] = true
	}
	for _, prop := range sortedKeysOfStringMap(find.Values) {
		if !matched[prop] {
			errs = append(errs, fmt.Sprintf(
				"webhooks: %q find.values names %q which is not in find.match", id, prop))
		}
	}
	return errs
}

// validateWebhookHeaders checks the header allowlist.
func validateWebhookHeaders(id string, hook Webhook) []string {
	var errs []string
	for _, h := range hook.Headers {
		if !webhookHeaderRegex.MatchString(h) {
			errs = append(errs, fmt.Sprintf(
				"webhooks: %q header %q is not a valid HTTP header name", id, h))
			continue
		}
		if isForbiddenWebhookHeader(h) {
			errs = append(errs, fmt.Sprintf(
				"webhooks: %q may not expose header %q (it carries credentials or a proxy-asserted identity)", id, h))
		}
	}
	if hook.MaxBodyBytes < 0 {
		errs = append(errs, fmt.Sprintf("webhooks: %q max_body_bytes must not be negative", id))
	}
	if hook.MaxBodyBytes > maxWebhookBodyCap {
		errs = append(errs, fmt.Sprintf(
			"webhooks: %q max_body_bytes %d exceeds the %d-byte ceiling", id, hook.MaxBodyBytes, maxWebhookBodyCap))
	}
	return errs
}

// validateWebhookRespond checks the response status is one a handler may send.
func validateWebhookRespond(id string, hook Webhook) []string {
	status := hook.Respond.Status
	if status == 0 {
		return nil
	}
	if status < http.StatusOK || status > 299 {
		return []string{fmt.Sprintf(
			"webhooks: %q respond.status %d must be a 2xx success status", id, status)}
	}
	return nil
}

// validateWebhookSteps checks each then: step is a well-formed tagged union.
func validateWebhookSteps(id string, hook Webhook, meta *metamodel.Metamodel) []string {
	var errs []string

	// A step runs against whichever entity the pipeline ended up with: the FOUND
	// one, or the one it CREATED. Those can be different types (a hook may find
	// `alpha` and create `beta`), and a step must therefore be valid on every
	// type it could reach — checking only one is wrong in both directions. It
	// rejects a legitimate hook whose step targets the created type, and it
	// admits a broken one whose step is invalid there, which then fails at
	// request time. That is precisely what the load-error contract exists to
	// prevent.
	stepTypes := webhookStepTypes(hook)

	for i, step := range hook.Then {
		set := 0
		if step.AppendSection != nil {
			set++
		}
		if len(step.Set) > 0 {
			set++
		}
		switch {
		case set == 0:
			errs = append(errs, fmt.Sprintf(
				"webhooks: %q then[%d] declares no action (expected append_section or set)", id, i))
			continue
		case set > 1:
			errs = append(errs, fmt.Sprintf(
				"webhooks: %q then[%d] declares more than one action (expected exactly one)", id, i))
			continue
		}

		if as := step.AppendSection; as != nil && strings.TrimSpace(as.Content) == "" {
			errs = append(errs, fmt.Sprintf(
				"webhooks: %q then[%d] append_section requires content", id, i))
		}

		for _, stepType := range stepTypes {
			if len(step.Set) == 0 || stepType == "" {
				continue
			}
			if def, ok := metaEntityDef(meta, stepType); ok {
				for _, prop := range sortedKeysOfStringMap(step.Set) {
					if _, known := def.Properties[prop]; !known {
						errs = append(errs, fmt.Sprintf(
							"webhooks: %q then[%d] sets unknown property %q on type %q", id, i, prop, stepType))
					}
				}
			}
		}
	}
	return errs
}

// webhookStepTypes returns every entity type a then: step could run against,
// deduplicated: the found type and the created type, which a find-or-create
// hook may declare independently.
//
// A property named by a step must exist on ALL of them. The pipeline does not
// know at load time which branch a given delivery will take, so "valid on the
// type we happen to check" is not a property worth enforcing — the step has to
// be valid whichever entity it lands on.
func webhookStepTypes(hook Webhook) []string {
	var types []string
	add := func(t string) {
		if t != "" && !slices.Contains(types, t) {
			types = append(types, t)
		}
	}
	if hook.Find != nil {
		add(hook.Find.Type)
	}
	add(hook.CreateType())
	return types
}

// metaEntityDef looks up an entity definition, tolerating a nil metamodel so
// config-only tests need not build one.
func metaEntityDef(meta *metamodel.Metamodel, typeName string) (*metamodel.EntityDef, bool) {
	if meta == nil {
		return nil, false
	}
	def, ok := meta.GetEntityDef(typeName)
	if !ok {
		return nil, false
	}
	return def, true
}

// sortedWebhookIDs returns hook ids in a deterministic order so validation
// errors are stable.
func sortedWebhookIDs(cfg *Config) []string {
	ids := make([]string, 0, len(cfg.Webhooks))
	for id := range cfg.Webhooks {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// sortedKeysOfStringMap returns a map's keys in sorted order.
func sortedKeysOfStringMap(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
