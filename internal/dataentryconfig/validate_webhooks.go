package dataentryconfig

import (
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"

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
// extraForbiddenWebhookHeadersMu guards it, because a process that builds
// several apps (a test binary, say) registers from several goroutines.
var (
	extraForbiddenWebhookHeaders   = map[string]bool{}
	extraForbiddenWebhookHeadersMu sync.RWMutex
)

// ForbidWebhookHeader registers a header name that webhook configs may not
// expose, in addition to the built-in list. Call it at wiring time, before
// config load. Case-insensitive; an empty name is ignored.
func ForbidWebhookHeader(name string) {
	if name = strings.ToLower(strings.TrimSpace(name)); name != "" {
		extraForbiddenWebhookHeadersMu.Lock()
		extraForbiddenWebhookHeaders[name] = true
		extraForbiddenWebhookHeadersMu.Unlock()
	}
}

// isForbiddenWebhookHeader reports whether a header name may not be exposed to
// a hook's templates. Name is compared lowercase against the exact-name list,
// the wiring-time registrations, and the family prefixes.
func isForbiddenWebhookHeader(name string) bool {
	lower := strings.ToLower(name)
	if forbiddenWebhookHeaders[lower] {
		return true
	}
	extraForbiddenWebhookHeadersMu.RLock()
	extra := extraForbiddenWebhookHeaders[lower]
	extraForbiddenWebhookHeadersMu.RUnlock()
	if extra {
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
// ERROR, matching how an automation condition: that fails to compile is treated;
// Mapping.Validate in package ingestspec gives the reason for the mapping half.
func validateWebhooks(cfg *Config, meta *metamodel.Metamodel) []string {
	var errs []string

	for _, id := range sortedWebhookIDs(cfg) {
		hook := cfg.Webhooks[id]
		if !webhookIDRegex.MatchString(id) {
			errs = append(errs, fmt.Sprintf(
				"webhooks: invalid hook ID %q (must match ^[a-z0-9][a-z0-9_-]{0,63}$; the ID is the URL segment)", id))
			continue
		}
		for _, msg := range hook.Mapping().Validate(meta) {
			errs = append(errs, fmt.Sprintf("webhooks: %q %s", id, msg))
		}
		errs = append(errs, validateWebhookHeaders(id, hook)...)
		errs = append(errs, validateWebhookRespond(id, hook)...)
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

// --- request-scoped actions (TKT-EFMRQM) -----------------------------------

// validateActionRequest checks an action's opt-in `request:` block.
//
// It shares the header floor and body ceiling with a declarative webhook
// deliberately rather than growing a second set: both surfaces expose an
// attacker-supplied request to operator-authored logic, so a header that is
// always wrong to expose on one is always wrong on the other, and a second
// list would drift the moment either gained a name. Every problem is a LOAD
// ERROR, matching validateWebhooks: an action that silently does not see the
// body it was written against loses a delivery a producer generally does not
// resend.
func validateActionRequest(id string, action Action, hasScript bool) []string {
	req := action.Request
	if req == nil {
		return nil
	}

	var errs []string
	if !hasScript {
		// A declarative `set:` action has no code to receive a request. Silently
		// ignoring the block would leave the operator believing a payload
		// reached logic that does not exist.
		errs = append(errs, fmt.Sprintf(
			"actions: %q declares request: requires script (a set: action has no script to receive it)", id))
	}

	for _, h := range req.Headers {
		if !webhookHeaderRegex.MatchString(h) {
			errs = append(errs, fmt.Sprintf(
				"actions: %q request header %q is not a valid HTTP header name", id, h))
			continue
		}
		if isForbiddenWebhookHeader(h) {
			errs = append(errs, fmt.Sprintf(
				"actions: %q may not expose header %q (it carries credentials or a proxy-asserted identity)", id, h))
		}
	}

	if req.MaxBodyBytes < 0 {
		errs = append(errs, fmt.Sprintf("actions: %q request max_body_bytes must not be negative", id))
	}
	if req.MaxBodyBytes > maxWebhookBodyCap {
		errs = append(errs, fmt.Sprintf(
			"actions: %q request max_body_bytes %d exceeds the %d-byte ceiling",
			id, req.MaxBodyBytes, maxWebhookBodyCap))
	}
	return errs
}
