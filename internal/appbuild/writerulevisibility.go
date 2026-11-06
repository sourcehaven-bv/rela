package appbuild

import (
	"fmt"
	"log/slog"
	"sort"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/writerule"
)

// warnWriteRules logs one warning per [WriteRuleVisibilityConflicts] entry.
//
// Nil: a nil policy or a schema whose rules do not compile logs nothing. The
// entity manager refuses the second at boot, with the compile error.
func warnWriteRules(meta *metamodel.Metamodel, policy *acl.Policy) {
	for _, msg := range WriteRuleVisibilityConflicts(meta, policy) {
		slog.Warn("appbuild: write-enforced validation reads a field-restricted property", "detail", msg)
	}
}

// WriteRuleVisibilityConflicts reports validation rules declared
// `enforce: write` that read a property some role can hide with `visible:`.
//
// # Why this warns rather than refuses
//
// Whether a write is refused depends on the values the rule reads, and the
// writer sees the outcome. When the rule reads a property the writer cannot
// read, each write tells the writer something about its value. A rule that
// compares it with a property the writer sets (`budget <= limit`) lets the
// writer narrow the hidden value down over several writes. The 422 message
// itself quotes no value; the channel is the refusal.
//
// The rule is operator-written, and many such rules are intended: a rule on
// a property a reader-only role cannot see discloses nothing to a writer who
// can. The check cannot tell those apart at load, where there is no
// principal, so it names the overlap and leaves the decision to the
// operator. It is pessimistic in the same way as
// [QueryScopeVisibilityConflicts]: any role that can hide the property
// counts.
//
// Returns one message per (rule, type, property) conflict, sorted.
//
// Nil: a nil metamodel or policy yields no conflicts.
func WriteRuleVisibilityConflicts(meta *metamodel.Metamodel, policy *acl.Policy) []string {
	if meta == nil || policy == nil {
		return nil
	}
	restricted := restrictedFields(policy, meta)
	if len(restricted) == 0 {
		return nil
	}
	set, err := writerule.Compile(meta)
	if err != nil {
		return nil
	}
	var out []string
	for _, r := range set.Reads() {
		hidden := restricted[r.Type]
		for _, prop := range r.Properties {
			roles, ok := hidden[prop]
			if !ok {
				continue
			}
			out = append(out, fmt.Sprintf(
				"entity %q: validation %q (enforce: write) reads %q, which role %s can hide with `visible:` — "+
					"a writer who holds only that role learns about the value from whether the write is refused",
				r.Type, r.Rule, prop, quotedRoleList(roles)))
		}
	}
	sort.Strings(out)
	return out
}
