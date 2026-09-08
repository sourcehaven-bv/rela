package metamodel

import (
	"fmt"
	"slices"
	"sort"
	"strings"
)

// CommentsConfig is the top-level `comments:` block: the operator's policy for
// the commentary layer (internal/comments).
//
// It carries POLICY ONLY — which entity types accept comments — and declares no
// schema. Comments are not graph entities, so there is no type here for the
// metamodel to define, validate or serve over /_schema. That separation is the
// point: a remark about a property is not a fact in the operator's domain model.
//
// Who may comment is a separate question answered by acl.yaml through the
// `comment:*` permissions, exactly as `history:read` gates version history.
// This block decides where commenting is POSSIBLE; the ACL decides who may do
// it.
type CommentsConfig struct {
	// Enabled turns commenting on. Absent or false means the feature does not
	// exist: no routes served, no storage touched.
	Enabled bool `yaml:"enabled"`

	// On lists the entity types that accept comments. The wildcard "*" means
	// every declared type.
	//
	// Empty with Enabled true is a load error rather than a silent
	// "everything" or a silent "nothing": both readings are defensible, so
	// guessing would make the config mean different things to different
	// readers.
	On []string `yaml:"on"`
}

// CommentWildcard makes every declared entity type commentable.
const CommentWildcard = "*"

// CommentPolicy is the resolved view over a metamodel's comment configuration.
//
// Constructed with [NewCommentPolicy] rather than living as methods on
// [Metamodel]: that type carries a plimsoll directive capping its exported
// surface, and a policy view is the established way to add accessors without
// growing it (the [AttachmentPolicy] precedent).
type CommentPolicy struct {
	meta *Metamodel
}

// NewCommentPolicy returns the comment-policy view over m.
//
// Nil: a nil m yields a policy that reports commenting as disabled, so callers
// on a partially-wired path degrade to "no commenting" rather than panicking.
func NewCommentPolicy(m *Metamodel) CommentPolicy {
	return CommentPolicy{meta: m}
}

// Enabled reports whether commenting is configured at all.
func (p CommentPolicy) Enabled() bool {
	return p.meta != nil && p.meta.Comments != nil && p.meta.Comments.Enabled
}

// Commentable reports whether entityType accepts comments.
//
// A type that is not declared in the metamodel is never commentable, even under
// the wildcard: the wildcard means "every type this project defines", not
// "any string a client sends".
func (p CommentPolicy) Commentable(entityType string) bool {
	if !p.Enabled() {
		return false
	}
	if _, ok := p.meta.GetEntityDef(entityType); !ok {
		return false
	}
	on := p.meta.Comments.On
	if slices.Contains(on, CommentWildcard) {
		return true
	}
	// Resolve through the alias map so a config naming a type by its alias
	// agrees with a request naming it canonically.
	canonical := p.meta.ResolveAlias(entityType)
	for _, t := range on {
		if t == entityType || p.meta.ResolveAlias(t) == canonical {
			return true
		}
	}
	return false
}

// CommentableTypes returns the commentable entity types, sorted.
//
// The wildcard is expanded here so callers never re-implement that resolution.
func (p CommentPolicy) CommentableTypes() []string {
	if !p.Enabled() {
		return nil
	}
	var out []string
	for name := range p.meta.Entities {
		if p.Commentable(name) {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

// validateComments checks the top-level `comments:` block.
//
// Like validateTransforms, a bad block fails the whole metamodel load: a
// half-working commenting config is worse than a refused one, because the
// failure would otherwise surface as comments silently not appearing on a type
// the operator believed they had enabled.
//
// It CANONICALIZES as it goes — trimming type names — so downstream consumers
// read resolved values and never re-trim.
func validateComments(m *Metamodel) []string {
	cfg := m.Comments
	if cfg == nil {
		return nil
	}

	// A block that names types but is not enabled is almost certainly a
	// half-finished edit. Report it rather than silently ignoring the list.
	if !cfg.Enabled {
		if len(cfg.On) > 0 {
			return []string{
				"comments: `on` lists types but `enabled` is false — " +
					"set `enabled: true` or remove the block",
			}
		}
		return nil
	}

	if len(cfg.On) == 0 {
		return []string{
			"comments: `on` is required when enabled — list the entity types " +
				`that accept comments, or use ["*"] for all of them`,
		}
	}

	var errs []string
	seen := map[string]bool{}
	cleaned := make([]string, 0, len(cfg.On))
	for _, raw := range cfg.On {
		name := strings.TrimSpace(raw)
		if name == "" {
			errs = append(errs, "comments: `on` contains an empty entity type")
			continue
		}
		if seen[name] {
			errs = append(errs, fmt.Sprintf("comments: `on` lists %q more than once", name))
			continue
		}
		seen[name] = true
		cleaned = append(cleaned, name)

		if name == CommentWildcard {
			continue
		}
		if _, ok := m.GetEntityDef(name); !ok {
			errs = append(errs, fmt.Sprintf(
				"comments: `on` names unknown entity type %q%s", name, didYouMeanEntity(m, name)))
		}
	}

	// The wildcard already covers everything, so naming types beside it is a
	// contradiction the operator should resolve rather than have silently
	// widened.
	if slices.Contains(cleaned, CommentWildcard) && len(cleaned) > 1 {
		errs = append(errs, `comments: `+"`on`"+` mixes "*" with named types — "*" already covers every type`)
	}

	sort.Strings(errs)
	m.Comments.On = cleaned
	return errs
}

// didYouMeanEntity returns a " (did you mean ...)" suffix when name is close to
// a declared entity type, or "" when nothing is close enough.
//
// A typo in `on:` is the most likely way this block goes wrong, and the failure
// it produces — comments quietly absent on one type — gives an operator no clue
// where to look.
func didYouMeanEntity(m *Metamodel, name string) string {
	lower := strings.ToLower(name)

	// Deterministic order: map iteration would make the hint vary run to run,
	// and an error message that changes between identical loads is worse than
	// no hint at all.
	candidates := make([]string, 0, len(m.Entities))
	for candidate := range m.Entities {
		candidates = append(candidates, candidate)
	}
	sort.Strings(candidates)

	for _, candidate := range candidates {
		lowerCandidate := strings.ToLower(candidate)
		// Substring in EITHER direction: the two most common typos are a
		// plural ("tickets" for "ticket") and a truncation ("tick"), and
		// checking only one direction catches only one of them.
		similar := lowerCandidate == lower ||
			strings.Contains(lowerCandidate, lower) ||
			strings.Contains(lower, lowerCandidate)
		if similar {
			return fmt.Sprintf(" (did you mean %q?)", candidate)
		}
	}
	return ""
}
