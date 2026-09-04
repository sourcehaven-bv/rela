// Lua binding for mail.render.
//
//	local html, text = mail.render{subject = ..., sections = {...}}
//
// This is the structured alternative to hand-writing HTML for mail.send.
// A script describes a message — subject, intro markdown, sections, a footer —
// and gets back the same branded, client-compatibility-hardened parts the
// declarative scheduled-mail path produces, rendered by internal/mailrender.
//
// Before this existed, a script that wanted formatted mail had exactly one
// option: assemble HTML by hand and pass it to mail.send. That reliably
// produces mail that looks right in the author's client and breaks in Outlook,
// because the rules are not obvious and nothing checks them. See TKT-1GA2PG.
//
// # Why this does not create an import cycle
//
// internal/mail depends on internal/lua (transport: script), and that arrow
// must never reverse. It does not here: this file imports internal/mailrender,
// a separate component that is a true LEAF — it imports nothing internal at
// all. The two arrows are independent, and arch-lint records the reasoning.
//
// # There is deliberately no raw-HTML or raw-CSS field
//
// Everything a script supplies is UNTRUSTED and goes through the same
// goldmark -> bluemonday -> template -> douceur pipeline as entity content, in
// that order, by reusing mailrender.Renderer wholesale rather than
// reimplementing any step.
//
// Adding an `html = ...` escape hatch here would reintroduce exactly the
// sanitizer bypass this binding exists to give authors an alternative to, and
// a `css = ...` field would hand untrusted values to douceur, which validates
// nothing and runs last. Neither field belongs here. If one seems necessary,
// the honest answer is still mail.send.
//
// # Errors raise
//
// Unlike mail.send, nothing here touches the network, so every failure is a
// malformed argument — a bug in the script that no retry fixes. That includes
// an invalid `lang`: a script that meant "nl" and typed something else should
// be told, not silently sent an English-labeled message.
package lua

import (
	"fmt"

	lua "github.com/yuin/gopher-lua"

	"github.com/Sourcehaven-BV/rela/internal/mailrender"
)

// BaseURLCarrier is an OPTIONAL capability a [MailSender] may implement to
// report the deployment's public base URL.
//
// An optional interface rather than a widened MailSender, for the reason
// [RecipientPolicyCarrier] documents.
//
// Where the two differ is the DEFAULT. An absent recipient policy must deny,
// because a transport that never considered the question is not an operator's
// blessing. An absent base URL is merely unknown, and unknown is safe here:
// the URL only affects ROOT-RELATIVE links in rendered mail. A sender that does
// not implement this yields an empty base, and mailrender's safeHref already
// handles that by dropping the relative link and rendering its text unlinked —
// so the failure mode is a missing hyperlink, never a broken or forged one.
type BaseURLCarrier interface {
	// MailBaseURL returns the public app URL, or "" if none is configured.
	MailBaseURL() string
}

// baseURLFor extracts the base URL a sender declares, if it declares one.
func baseURLFor(sender MailSender) string {
	if carrier, ok := sender.(BaseURLCarrier); ok {
		return carrier.MailBaseURL()
	}
	return ""
}

// mailRenderFunc returns the mail.render binding bound to sender.
//
// A FREE function returning a closure, rather than a method on Runtime like
// mail.send. Two reasons, and the first is the load-bearing one:
//
//   - Runtime sits at its plimsoll load line (//plimsoll:max-methods=46), and
//     the sanctioned fix for a full type is to take methods off it (TKT-N0IKN9),
//     not to raise the cap. A binding that needs one field is not a reason to
//     widen the type's surface.
//   - The contextcheck false positive that forces mail.send to register as a
//     method VALUE does not apply here: nothing in this path takes or threads a
//     context, because rendering is pure formatting with no I/O.
//
// Registered unconditionally and usable with no mail transport configured:
// "mail is not configured" would be a nonsense answer to a request that never
// needed a transport. A script may legitimately render a message to log it,
// diff it, or hand it somewhere other than mail.send. A nil sender simply
// yields an empty base URL.
func mailRenderFunc(sender MailSender) lua.LGFunction {
	return func(ls *lua.LState) int {
		opts := ls.CheckTable(1)

		msg, err := parseRenderMessage(opts)
		if err != nil {
			ls.RaiseError("mail.render: %s", err.Error())
			return 0
		}

		renderer, err := mailrender.New(&mailrender.Options{BaseURL: baseURLFor(sender)})
		if err != nil {
			ls.RaiseError("mail.render: %s", err.Error())
			return 0
		}

		html, text, err := renderer.Render(msg)
		if err != nil {
			ls.RaiseError("mail.render: %s", err.Error())
			return 0
		}

		ls.Push(lua.LString(html))
		ls.Push(lua.LString(text))
		return 2
	}
}

// parseRenderMessage converts the Lua opts table into a mailrender.Message.
func parseRenderMessage(opts *lua.LTable) (*mailrender.Message, error) {
	subject, err := requiredString(opts, "subject")
	if err != nil {
		return nil, err
	}

	msg := &mailrender.Message{Subject: subject}
	for field, dst := range map[string]*string{
		"intro":  &msg.Intro,
		"footer": &msg.Footer,
		"lang":   &msg.Lang,
	} {
		v, optErr := optionalString(opts, field)
		if optErr != nil {
			return nil, optErr
		}
		*dst = v
	}

	sections, err := parseRenderSections(opts.RawGetString("sections"))
	if err != nil {
		return nil, err
	}
	msg.Sections = sections
	return msg, nil
}

// parseRenderSections converts the `sections` array.
//
// An absent `sections` is fine — a message that is just an intro and a footer
// is a legitimate notification — but a present-and-wrong one is an error rather
// than a silent empty, because a script author who wrote `sections = {...}` and
// got no sections deserves to be told which value was rejected.
func parseRenderSections(v lua.LValue) ([]mailrender.Section, error) {
	if v == lua.LNil {
		return nil, nil
	}
	tbl, ok := v.(*lua.LTable)
	if !ok {
		return nil, fmt.Errorf("sections must be a table, got %s", v.Type().String())
	}

	n := tbl.Len()
	out := make([]mailrender.Section, 0, n)
	for i := 1; i <= n; i++ {
		raw := tbl.RawGetInt(i)
		st, stOK := raw.(*lua.LTable)
		if !stOK {
			return nil, fmt.Errorf("sections[%d] must be a table, got %s", i, raw.Type().String())
		}
		s, err := parseRenderSection(st, i)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}

func parseRenderSection(tbl *lua.LTable, idx int) (mailrender.Section, error) {
	var s mailrender.Section

	for field, dst := range map[string]*string{"title": &s.Title, "body": &s.Body} {
		v, err := optionalString(tbl, field)
		if err != nil {
			return s, fmt.Errorf("sections[%d]: %w", idx, err)
		}
		*dst = v
	}

	cols, err := stringArray(tbl.RawGetString("columns"), "columns")
	if err != nil {
		return s, fmt.Errorf("sections[%d]: %w", idx, err)
	}
	s.Columns = cols

	links, err := stringArray(tbl.RawGetString("links"), "links")
	if err != nil {
		return s, fmt.Errorf("sections[%d]: %w", idx, err)
	}
	s.Links = links

	rows, err := parseRenderRows(tbl.RawGetString("rows"), idx)
	if err != nil {
		return s, err
	}
	s.Rows = rows
	return s, nil
}

func parseRenderRows(v lua.LValue, idx int) ([][]string, error) {
	if v == lua.LNil {
		return nil, nil
	}
	tbl, ok := v.(*lua.LTable)
	if !ok {
		return nil, fmt.Errorf("sections[%d]: rows must be a table, got %s", idx, v.Type().String())
	}

	n := tbl.Len()
	out := make([][]string, 0, n)
	for i := 1; i <= n; i++ {
		raw := tbl.RawGetInt(i)
		// A hole is rejected rather than skipped. stringArray treats LNil as
		// "this optional array was omitted", which is right for `columns` and
		// `links` but meaningless for a row SLOT: a row is an element, not an
		// option. Delegating here would append a zero-cell row and emit a bare
		// <tr></tr> between the real ones — a malformed table with no error.
		//
		// Lua's `#` is undefined on a table with a hole, so the count itself is
		// unreliable; failing loudly is the only honest answer.
		if raw == lua.LNil {
			return nil, fmt.Errorf(
				"sections[%d].rows[%d] is missing — a nil hole in the array", idx, i)
		}
		row, err := stringArray(raw, "row")
		if err != nil {
			return nil, fmt.Errorf("sections[%d].rows[%d]: %w", idx, i, err)
		}
		out = append(out, row)
	}
	return out, nil
}

// stringArray converts a Lua array of strings. A nil value yields nil, so an
// omitted optional array is not an error.
//
// Cell values must already BE strings: a silent tostring() would render
// "1.0" for a number a script author wrote as 1, and guessing a format for
// someone else's data is worse than making them choose one.
func stringArray(v lua.LValue, field string) ([]string, error) {
	if v == lua.LNil {
		return nil, nil
	}
	tbl, ok := v.(*lua.LTable)
	if !ok {
		return nil, fmt.Errorf("%s must be a table, got %s", field, v.Type().String())
	}

	n := tbl.Len()
	out := make([]string, 0, n)
	for i := 1; i <= n; i++ {
		item := tbl.RawGetInt(i)
		s, sok := item.(lua.LString)
		if !sok {
			return nil, fmt.Errorf("%s[%d] must be a string, got %s", field, i, item.Type().String())
		}
		out = append(out, string(s))
	}
	return out, nil
}

func requiredString(tbl *lua.LTable, field string) (string, error) {
	s, ok := tbl.RawGetString(field).(lua.LString)
	if !ok {
		return "", fmt.Errorf("%s must be a string", field)
	}
	return string(s), nil
}

func optionalString(tbl *lua.LTable, field string) (string, error) {
	v := tbl.RawGetString(field)
	if v == lua.LNil {
		return "", nil
	}
	s, ok := v.(lua.LString)
	if !ok {
		return "", fmt.Errorf("%s must be a string, got %s", field, v.Type().String())
	}
	return string(s), nil
}
