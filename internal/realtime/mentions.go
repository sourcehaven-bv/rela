package realtime

import (
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/mentions"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/realtimecfg"
)

// maxCandidateSpans is how many candidate spans of one message are
// considered, in document order. It bounds the ids one post adds to its
// Resolve batch.
const maxCandidateSpans = 64

// MaxMentions is how many people and agents one message may address
// ([channels.MaxMentions]), for transports that do not import channels.
const MaxMentions = channels.MaxMentions

// loginPrefix starts a login mention: `@jeroen` names the login jeroen.
const loginPrefix = "@"

// classifySpans sorts the code spans of a message into mention candidates.
// userType is the mention user type ([realtimecfg.Compiled.MentionUserType]).
// When it is set, a span that is an entity id is a user candidate, which
// the caller resolves; when it is empty, a span `@x` is a login target x
// (see [loginTarget]). A job identity ([principal.ValidJobIdentity]) is a
// job target either way. Any other span is plain code. Only the first
// [maxCandidateSpans] candidates count. The three forms cannot collide: an
// entity id holds neither ':' nor '@'.
//
// Nil: users and others are nil when no span is a candidate.
func classifySpans(spans []string, userType string) (users []string, others []channels.Mention) {
	n := 0
	for _, s := range spans {
		if n == maxCandidateSpans {
			break
		}
		switch {
		case principal.ValidJobIdentity(s) == nil:
			others = append(others, channels.Mention{Target: s, Form: channels.FormJob})
		case userType != "" && entity.ValidateID(s) == nil:
			users = append(users, s)
		case userType == "" && strings.HasPrefix(s, loginPrefix):
			login, ok := loginTarget(strings.TrimPrefix(s, loginPrefix))
			if !ok {
				continue
			}
			others = append(others, channels.Mention{Target: login, Form: channels.FormLogin, Span: s})
		default:
			continue
		}
		n++
	}
	return users, others
}

// loginTarget reports whether x may be a login mention target: 1 to
// [channels.MaxTextBytes] bytes of NFC UTF-8 with no whitespace, control
// character, Unicode format character (category Cf, such as U+200B or a
// bidi mark), backtick or NUL, and neither [principal.Unknown] nor reserved
// ([principal.IsReserved]). The match is exact and case-sensitive, as
// acl.yaml assignments are.
func loginTarget(x string) (string, bool) {
	if x == "" || len(x) > channels.MaxTextBytes || !utf8.ValidString(x) || !norm.NFC.IsNormalString(x) {
		return "", false
	}
	for _, r := range x {
		if unicode.IsSpace(r) || unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || r == '`' {
			return "", false
		}
	}
	if x == principal.Unknown || principal.IsReserved(x) {
		return "", false
	}
	return x, true
}

// markdownSpans returns the code spans of the markdown properties of mt in
// props, in a fixed order: by property name, then document order. Strings
// and enums are never parsed.
//
// Nil: returned when no markdown property holds a span.
func markdownSpans(mt *realtimecfg.MessageType, props map[string]any) []string {
	names := make([]string, 0, len(mt.Properties))
	for name, p := range mt.Properties {
		if p.Type == realtimecfg.PropertyMarkdown {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	var out []string
	for _, name := range names {
		if body, ok := props[name].(string); ok {
			out = append(out, mentions.Spans(body)...)
		}
	}
	return out
}

// mentionPlan is what a message's properties name as mention candidates,
// before resolution.
type mentionPlan struct {
	// users are the user candidate spans; the caller resolves them.
	users []string
	// others are the job and login targets.
	others []channels.Mention
	// refs are the ids of the ref properties that may notify: those
	// without `notify: false`.
	refs []string
}

// planMentions finds the mention candidates of a message of type mt with
// props, under mention user type userType.
func planMentions(mt *realtimecfg.MessageType, props map[string]any, userType string) mentionPlan {
	var p mentionPlan
	p.users, p.others = classifySpans(markdownSpans(mt, props), userType)
	if userType == "" {
		return p
	}
	for name, prop := range mt.Properties {
		if prop.Type != realtimecfg.PropertyRef || prop.NoNotify {
			continue
		}
		if id, ok := props[name].(string); ok && id != "" {
			p.refs = append(p.refs, id)
		}
	}
	return p
}

// ids returns every id the plan needs resolved.
func (p mentionPlan) ids() []string {
	return append(slices.Clone(p.users), p.refs...)
}

// targets returns the mention targets of p given the resolved refs: each
// user candidate and notifying ref that resolved to an entity of userType,
// stored as its canonical id, then the job and login targets, without the
// poster's own key self. The result is deduplicated and sorted.
//
// Nil: returned when nothing is addressed.
func (p mentionPlan) targets(got map[string]RefView, userType, self string) []channels.Mention {
	seen := map[string]bool{}
	var out []channels.Mention
	add := func(m channels.Mention) {
		if m.Target == self || seen[m.Target] {
			return
		}
		seen[m.Target] = true
		out = append(out, m)
	}
	for _, id := range p.ids() {
		ref, ok := got[id]
		if !ok || userType == "" || ref.Type != userType {
			continue
		}
		m := channels.Mention{Target: ref.ID, Form: channels.FormUser}
		if id != ref.ID {
			m.Span = id
		}
		add(m)
	}
	for _, m := range p.others {
		add(m)
	}
	slices.SortFunc(out, func(a, b channels.Mention) int { return strings.Compare(a.Target, b.Target) })
	return out
}

// candidate reports whether target m is still named by p: a user target
// whose id or span is a user candidate or a notifying ref, or a job or
// login target p holds. An edit keeps such a target even when the editor
// can no longer read it (see [Service.Edit]).
func (p mentionPlan) candidate(m channels.Mention) bool {
	if m.Form == channels.FormUser {
		for _, id := range p.ids() {
			if id == m.Target || (m.Span != "" && id == m.Span) {
				return true
			}
		}
		return false
	}
	return slices.ContainsFunc(p.others, func(o channels.Mention) bool { return o.Target == m.Target })
}
