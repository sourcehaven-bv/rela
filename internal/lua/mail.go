// Lua bindings for the top-level mail.* module.
//
//	mail.send{to = ..., subject = ..., html = ..., text = ...}
//	  -> (true, nil) | (nil, err_table)
//	mail.render{subject = ..., sections = {...}} -> html, text
//
// mail.render is the structured alternative to composing `html` by hand; it
// lives in mailrender.go, which documents why it exists and why it has no
// raw-HTML field.
//
// The binding is a THIN pass to the configured [MailSender]. It does not
// render, does not template, and does not know a transport from a hole in the
// ground — it hands a message to the same seam every other mail caller uses,
// so a script cannot reach a delivery path the operator did not configure, and
// cannot send through a transport the project has switched off.
//
// # Two separate questions: may it be CALLED, and may it SEND
//
// mail.send is subject to both, and they are answered in different places for
// different reasons. Conflating them is what left the exfiltration hole this
// package doc used to argue was not one (TKT-JVHSOZ).
//
// ## Always registered, never silently absent
//
// Unlike ai.* and http.*, the `mail` global exists on every runtime, whether
// or not mail is configured and whether or not the script may send. That is
// deliberate, and the reasoning is about ERGONOMICS, not authorization:
//
// A binding that vanished when mail.yaml was absent would make
// `mail.send(...)` raise "attempt to call a nil value" — a message that says
// nothing about mail — in exactly the deployment where the operator most needs
// to be told "mail is not configured". So the function is always there and
// returns a typed error instead, which a script can feature-detect on:
//
//	local ok, err = mail.send{...}
//	if not ok and err.kind == "not_configured" then ... end
//
// That argument holds with MORE force for a denied capability, not less: a nil
// global cannot name the YAML key an operator has to add, and a denial is
// exactly the moment they need to be told which one.
//
// ## Authorized by [Capabilities.Mail]
//
// The registration argument above is about the quality of an error message. It
// says nothing about WHO may send, and for a long time nothing else did
// either: the binding was reachable from every runtime, so any script holding
// `rela.secrets` could mail a credential to an address of its choosing without
// holding `http` or `ai`. That is the same two-call exfiltration path those two
// are gated to close (TKT-YH52OM), reached through a door nobody had gated.
//
// The old rationale for leaving it open was that "mail.send is not a capability
// the script holds; it is a service the PROJECT either has or has not
// configured". The first half of that is what does not survive: it establishes
// that the TRANSPORT is operator configuration — a script cannot invent an SMTP
// server — but not that every script may reach it. The operator asymmetry is
// the whole point of the capability system. An unexpected `mail: true` on a
// 500-line script stands out in a config file an operator reviews; the same
// reach buried in that script's code does not.
//
// So: registered unconditionally, refuses without the grant. A denied call
// returns `err.kind == "denied"` and NOTHING is handed to the sender.
//
// The grant is checked FIRST, before the sender lookup and before the argument
// table is parsed. An unauthorized script therefore cannot learn whether the
// project has mail configured at all — and, more practically, an operator
// debugging a denial is pointed at their `capabilities:` block rather than at
// a mail.yaml that has nothing wrong with it.
//
// # Error convention
//
// Delivery is network-bound, so a failure is an EXPECTED runtime condition and
// returns (nil, err_table) per the ai.*/http.* convention. It never raises: a
// script that mails a summary at the end of a run must not lose the run
// because the mail server was rebooting. Raising is reserved for argument
// errors — a missing recipient is a bug in the script, not a fact about the
// world.
package lua

import (
	"context"
	"errors"
	"fmt"

	lua "github.com/yuin/gopher-lua"
)

// MailSender is the consumer-side view of a mail transport, declared here
// rather than imported from internal/mail so this package does not depend on
// that one (see CLAUDE.md: interfaces at the call site). *mail.MemorySender,
// *mail.SMTPSender, *mail.HTTPSender and the outbox all satisfy it
// structurally; the wiring site supplies whichever one the project configured.
type MailSender interface {
	SendMail(ctx context.Context, msg MailMessage) error
}

// MailMessage is a message handed to a [MailSender] from Lua.
//
// A struct rather than a map so the field set is a compile-time contract with
// the wiring site: an adapter that forgot to carry Text fails to build rather
// than sending half a message.
type MailMessage struct {
	// To are the recipient addresses. At least one is required.
	To []string

	// Subject is the mail subject.
	Subject string

	// HTML is the text/html part.
	HTML string

	// Text is the text/plain part.
	Text string
}

// errMailNotConfigured is the sentinel a wiring site can return, and the
// condition this package reports when no sender was wired at all.
var errMailNotConfigured = errors.New("mail is not configured for this project")

// errMailDenied is what an ungranted mail.send reports (TKT-JVHSOZ).
//
// The message names the exact YAML key to add, because the operator reading it
// is the one who can fix it and the fix is one line. It deliberately says
// nothing about whether mail is configured, what transport is in use, or who
// the caller is: a script that was not authorized to send has not earned any
// facts about the deployment, and the omission costs a legitimate operator
// nothing they cannot see in their own config.
var errMailDenied = errors.New(
	"mail.send is not permitted: this script has no `mail` capability " +
		"(add `mail: true` to its `capabilities:` block)")

// registerMailModule installs the top-level `mail` global.
//
// A free function rather than a *Runtime method, matching crypto.go: Runtime
// sits at its plimsoll load line, and the fix for a full type is to take
// fields off it (TKT-N0IKN9), not to route new methods around the counter.
func registerMailModule(r *Runtime) {
	tbl := r.L.NewTable()
	// A METHOD VALUE, not a closure, and that is not stylistic. contextcheck
	// follows a closure back through registerBindings to every
	// NewReader/NewWriter call site and demands each thread a context into
	// runtime CONSTRUCTION — which is meaningless: a binding runs later, when
	// a script calls it, and the right context is the runtime's own. The
	// analyzer does not follow method values, which is why ai.* and http.*
	// register theirs this way and are not flagged. Matching them is the
	// substantive fix; suppressing the finding at twelve unrelated call sites
	// would be the cosmetic one.
	r.L.SetField(tbl, "send", r.L.NewFunction(r.luaMailSend))
	// mail.render is pure formatting with no transport, so it is registered on
	// the same terms as send and works even when mail is unconfigured — see
	// the doc on mailrender.go.
	r.L.SetField(tbl, "render", r.L.NewFunction(mailRenderFunc(r.mailSender)))
	r.L.SetGlobal("mail", tbl)
}

// mailSendFunc returns the mail.send binding bound to r.
//
// A named constructor rather than an inline closure so the binding reads the
// same way registerHTTPModule's method values do, and so the context handling
// below is a plain function body rather than a lambda the reader has to
// unpick.
// luaMailSend implements mail.send(opts) -> (true, nil) | (nil, err_table).
//
// opts fields:
//
//	to      (string or array of strings, required)
//	subject (string, required)
//	html    (string, optional)
//	text    (string, optional)
//
// At least one of html/text is required; that check is left to the sender so
// this binding and every other mail caller agree on what a valid message is
// rather than having two definitions that can drift.
func (r *Runtime) luaMailSend(ls *lua.LState) int {
	// The capability check comes FIRST — ahead of the sender lookup and ahead
	// of parsing the opts table. Ordering is load-bearing twice over: an
	// ungranted script learns nothing about whether the project has mail
	// configured, and an operator debugging a denial is sent to the
	// `capabilities:` block rather than to a mail.yaml that is fine. Parsing
	// first would also let a denied caller distinguish a valid message from an
	// invalid one by whether the call raised.
	if !r.caps.Mail {
		return pushMailError(ls, "denied", errMailDenied.Error())
	}

	opts := ls.CheckTable(1)

	sender := r.mailSender
	if sender == nil {
		return pushMailError(ls, "not_configured", errMailNotConfigured.Error())
	}

	msg, err := parseMailMessage(opts)
	if err != nil {
		// A malformed call is a programming error, so it RAISES — the script
		// author can fix it and no amount of retrying will. Contrast the
		// delivery failures below, which the world causes and the script may
		// reasonably want to handle.
		ls.RaiseError("mail.send: %s", err.Error())
		return 0
	}

	// Enforce the operator's recipients allowlist BEFORE handing anything to
	// the transport (TKT-USQNA3). A denial is a returned error, not a raise:
	// it is a fact about configuration the script may reasonably handle, in
	// the same class as a delivery failure rather than a malformed call.
	if err := checkRecipients(recipientPolicyFor(sender), msg.To); err != nil {
		return pushMailError(ls, recipientDeniedKind, err.Error())
	}

	// callerCtx is the runtime's caller context — the same one the write
	// bindings use for audit attribution — so a send inherits the caller's
	// cancellation. It is the LState's context that carries the script
	// TIMEOUT; that bounds the script as a whole, and bounding an individual
	// send by it would make a long report's last mail fail for reasons that
	// have nothing to do with the mail server. The transport applies its own
	// send timeout from mail.yaml.
	ctx := r.callerCtx()

	if sendErr := sender.SendMail(ctx, msg); sendErr != nil {
		return pushMailError(ls, classifyMailError(ctx, sendErr), sendErr.Error())
	}

	ls.Push(lua.LTrue)
	ls.Push(lua.LNil)
	return 2
}

// classifyMailError buckets a delivery failure into an err.kind a script can
// branch on.
//
// Three kinds, not a taxonomy of SMTP codes: a script's only real decisions
// are "was this my fault", "is retrying pointless because it will never be
// configured", and "did we run out of time". Finer classification belongs to
// the outbox, which is what actually retries.
func classifyMailError(ctx context.Context, err error) string {
	if errors.Is(err, errMailNotConfigured) {
		return "not_configured"
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}
	// A cancelled context with an unrelated error underneath still means the
	// send ran out of time — the transport just reported the symptom (a closed
	// connection, say) rather than the cause.
	if ctx != nil && ctx.Err() != nil {
		return "timeout"
	}
	return "delivery_failed"
}

// parseMailMessage converts the Lua opts table into a MailMessage.
func parseMailMessage(opts *lua.LTable) (MailMessage, error) {
	var out MailMessage

	to, err := parseMailRecipients(opts.RawGetString("to"))
	if err != nil {
		return out, err
	}
	out.To = to

	subject, ok := opts.RawGetString("subject").(lua.LString)
	if !ok {
		return out, errors.New("subject must be a string")
	}
	out.Subject = string(subject)

	if v := opts.RawGetString("html"); v != lua.LNil {
		s, sok := v.(lua.LString)
		if !sok {
			return out, fmt.Errorf("html must be a string, got %s", v.Type().String())
		}
		out.HTML = string(s)
	}
	if v := opts.RawGetString("text"); v != lua.LNil {
		s, sok := v.(lua.LString)
		if !sok {
			return out, fmt.Errorf("text must be a string, got %s", v.Type().String())
		}
		out.Text = string(s)
	}

	return out, nil
}

// parseMailRecipients accepts either a single address string or an array of
// them. Both shapes, because `to = "a@example.com"` is what a script writes
// nine times out of ten and forcing a one-element table there is friction with
// no payoff.
func parseMailRecipients(v lua.LValue) ([]string, error) {
	switch t := v.(type) {
	case lua.LString:
		if t == "" {
			return nil, errors.New("to must not be empty")
		}
		return []string{string(t)}, nil
	case *lua.LTable:
		n := t.Len()
		if n == 0 {
			return nil, errors.New("to must contain at least one address")
		}
		out := make([]string, 0, n)
		for i := 1; i <= n; i++ {
			s, ok := t.RawGetInt(i).(lua.LString)
			if !ok || s == "" {
				return nil, fmt.Errorf("to[%d] must be a non-empty string", i)
			}
			out = append(out, string(s))
		}
		return out, nil
	default:
		return nil, errors.New("to must be a string or an array of strings")
	}
}

// pushMailError pushes (nil, err_table) in the ai.*/http.* shape.
//
// The message is passed through verbatim from the transport. That is safe
// because internal/mail redacts credentials from its own error strings before
// they escape (see mail.redact) — a property that transport owns, since only
// it knows what its credential is.
func pushMailError(ls *lua.LState, kind, message string) int {
	ls.Push(lua.LNil)
	tbl := ls.NewTable()
	tbl.RawSetString("kind", lua.LString(kind))
	tbl.RawSetString("message", lua.LString(message))
	tbl.RawSetString("retry_after", lua.LNumber(0))
	tbl.RawSetString("details", lua.LString(""))
	ls.Push(tbl)
	return 2
}
