package mail

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	glua "github.com/yuin/gopher-lua"

	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/secrets"
)

// ScriptSender delivers by running an operator-supplied Lua script.
//
// # Why a script rather than a mapping DSL
//
// Provider send APIs agree on nothing — encoding, auth scheme, sender shape,
// recipient shape and body field names all differ, and Mailgun is not JSON at
// all (multipart/form-data with HTTP Basic). A JSON field-mapping DSL could
// cover the three JSON providers and would exclude Mailgun by construction, so
// the general answer is a script plus general HTTP primitives. This follows
// FEAT-CN5L0X: the generic primitive lives in Go, the provider specifics live
// in an example Lua script (see examples/mail/).
//
// # The script has NO graph access, by construction
//
// The runtime is built with a ZERO [lua.ReadDeps] and is a READER, so:
//
//   - rela.get_entity, rela.list_entities, rela.search and every traversal
//     binding are registered against nil collaborators and raise rather than
//     read;
//   - no mutation binding is registered at all — the runtime is not a writer,
//     so create/update/delete are absent from the rela table;
//   - rela.secrets contains only the keys `capabilities.secrets` names.
//
// This is enforced by what is wired, not by a check the script could be
// written around, and it is asserted in script_test.go rather than only
// claimed here. The script receives an already-rendered message and can do
// exactly one useful thing with it: ship it. Rendering was ACL-gated upstream
// (TKT-U2R7GU), so it never needs graph access to do its job.
//
// # Trust model
//
// A send script is TRUSTED CODE, the same posture internal/ai states for Lua
// generally. It holds an outbound-HTTP capability and a named credential, so
// an operator who installs a hostile script has already lost — capability
// gating narrows the blast radius (it cannot read the graph, and it sees only
// the secrets it named), it does not make an untrusted script safe to run.
// Treat `script:` in mail.yaml as you would treat a systemd unit file.
type ScriptSender struct {
	cfg Config

	// scriptPath is the absolute path to the operator's script. Resolved once
	// at construction so a later working-directory change cannot make delivery
	// start failing halfway through a process's life.
	scriptPath string

	// secretsScope is the key `secrets.Load` is called with. See
	// [ScriptSender.buildRuntime] for why it is the script path and not the
	// triggering principal.
	secretsScope string

	// relaDir is where secrets.yaml is read from.
	relaDir string

	// stdout receives anything the script prints. Discarded by default: a mail
	// transport running on a background worker has no terminal, and print()
	// output interleaved into a server's stdout is noise, not diagnostics.
	stdout io.Writer
}

// ScriptOption adjusts a ScriptSender.
type ScriptOption func(*ScriptSender)

// WithScriptStdout routes the script's print() output somewhere. For tests and
// for `rela mail test`-style operator debugging.
func WithScriptStdout(w io.Writer) ScriptOption {
	return func(s *ScriptSender) {
		if w != nil {
			s.stdout = w
		}
	}
}

// NewScriptSender returns a sender that delivers by running cfg.Script.
//
// Nil: a nil config is rejected. The config is re-validated here rather than
// trusted from load, so a programmatically constructed Config cannot bypass
// the checks the YAML path enforces.
func NewScriptSender(cfg *Config, opts ...ScriptOption) (*ScriptSender, error) {
	if cfg == nil {
		return nil, errors.New("mail: nil config")
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if cfg.Transport != TransportScript {
		return nil, fmt.Errorf("mail: NewScriptSender given transport %q", cfg.Transport)
	}

	s := &ScriptSender{
		cfg: *cfg,
		// The scope is the path as WRITTEN in mail.yaml, not the absolute one:
		// secrets.yaml `overrides:` keys are project-relative script paths, so
		// scoping on the resolved absolute path would never match an override
		// and the operator's per-script credential would be silently ignored.
		secretsScope: cfg.Script,
		relaDir:      cfg.relaDir,
		stdout:       io.Discard,
	}
	s.scriptPath = cfg.resolveScriptPath()
	for _, o := range opts {
		o(s)
	}
	return s, nil
}

// SendScriptPrincipal is the identity a send script runs as.
//
// # The worker-context decision (TKT-DS1CR6)
//
// The outbox worker has no triggering principal: it delivers minutes after the
// enqueue, on a retry, on a background goroutine, with the request that
// produced the message long gone. So "who is sending this" has to be answered
// deliberately rather than left to whatever happens to be in scope.
//
// Two candidates were considered and one rejected:
//
// REJECTED — carry the enqueuing principal on the Message and run the script
// as them. It reads like better attribution and is worse in every respect that
// matters. The credential the script uses is the OPERATOR's, not the user's,
// so the audit line would name a person who has no relationship to it and
// cannot revoke it; a per-user secrets scope would make delivery succeed or
// fail depending on who happened to trigger it, which is a nondeterministic
// mail system; and Message.RenderedFor already records the identity that
// actually matters — the one whose visibility bounded the CONTENT. Attribution
// of the delivery is a different question from attribution of the content, and
// conflating them loses the second.
//
// CHOSEN — a fixed system principal, with the secrets scope keyed on the
// configured script path. Delivery is infrastructure the operator configured
// once, in a file only they can write, and every send through a given
// transport is the same act regardless of what triggered it. The script path
// is the right scope key because it is what `secrets.Load` already keys on
// everywhere else, so an operator writes the same `overrides:` block they
// would write for any other script and it works — no second, mail-only
// convention to learn. The path is also stable across restarts and retries,
// which a principal is not.
//
// The concrete consequence: `mail.yaml`'s `script: mail/mailgun.lua` picks up
// `overrides: {"mail/mailgun.lua": {...}}` from secrets.yaml, and the audit
// trail for a delivery reads `system:mail`.
const SendScriptPrincipal = "system:mail"

// SendScriptTool is the tool half of the send script's identity. `principal`
// carries both, and leaving Tool empty would render as the documented
// "unknown" fallback — indistinguishable, in a log, from an unstamped context.
const SendScriptTool = "mail"

// Send runs the script with the rendered message in `rela.args`.
//
// Validation runs first and independently of the script, so this transport
// refuses exactly the messages SMTP, memory and HTTP refuse — the property the
// mailtest conformance suite exists to hold. A script cannot opt out of the
// header-injection check by being written differently.
func (s *ScriptSender) Send(ctx context.Context, m Message) error {
	if err := m.Validate(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	rt, err := s.buildRuntime(ctx)
	if err != nil {
		return err
	}
	defer rt.Close()

	// The message is set as a GLOBAL rather than passed as an argument because
	// rela.args is a string array by contract and a message is structured.
	// Named `message` singular, and the script is called once per message, so
	// a script cannot accidentally batch and lose the per-recipient ACL
	// scoping the render was done under.
	rt.LState().SetGlobal("message", s.messageTable(rt.LState(), m))

	// The context IS threaded — buildRuntime passes lua.WithContext(ctx), and
	// RunFile's applyTimeout derives the LState's context from it — but that
	// flow crosses a functional option, which contextcheck cannot follow. The
	// same suppression, for the same reason, appears in internal/script and
	// internal/docs.
	//nolint:contextcheck // ctx threaded via lua.WithContext in buildRuntime.
	if runErr := rt.RunFile(s.scriptPath, nil); runErr != nil {
		// A script failure is an ordinary delivery failure: it returns an
		// error, the outbox retries it through the existing backoff ladder,
		// and after MaxAttempts it is logged and dropped. Nothing here
		// duplicates a message — the outbox is the only thing that re-sends,
		// and it re-sends the same immutable Message.
		return fmt.Errorf("mail: send script %s: %w", s.cfg.Script, runErr)
	}
	return nil
}

// buildRuntime constructs the sandboxed runtime the script runs in.
//
// Note what is NOT passed: no Store, no Tracer, no Searcher, no Meta, no
// EntityManager. The zero ReadDeps is the whole security argument — see the
// type doc — and it is a reader, so no mutation binding is registered either.
func (s *ScriptSender) buildRuntime(ctx context.Context) (*lua.Runtime, error) {
	caps := s.cfg.Capabilities.toLua()

	opts := []lua.Option{
		lua.WithContext(ctx),
		lua.WithCapabilities(caps),
		lua.WithPrincipal(principal.Principal{User: SendScriptPrincipal, Tool: SendScriptTool}),
	}

	// Secrets are loaded under the SCRIPT-PATH scope, per the decision
	// recorded on SendScriptPrincipal, and then filtered again by the
	// capability grant inside the runtime — so an operator who lists
	// `secrets: [mailgun_key]` gets exactly that key even though secrets.yaml
	// holds the database DSN too.
	sec, err := secrets.Load(s.relaDir, s.secretsScope)
	switch {
	case errors.Is(err, secrets.ErrNotFound):
		// No secrets file. Not an error: a script may authenticate from an
		// environment variable, or need no credential at all.
	case err != nil:
		return nil, fmt.Errorf("mail: loading secrets for send script: %w", err)
	default:
		opts = append(opts, lua.WithSecrets(sec))
	}

	// A ZERO ReadDeps: no store, no tracer, no searcher, no metamodel. The
	// graph bindings are registered but have nothing behind them, so they
	// raise. NewReader, never NewWriter — a writer would additionally register
	// create/update/delete.
	return lua.NewReader(lua.ReadDeps{}, s.stdout, opts...), nil
}

// messageTable renders m as the Lua `message` global.
//
// Flat and provider-neutral: to/subject/html/text/rendered_for plus inline
// images. The script's job is to map these onto whatever field names its
// provider wants, which is exactly the mapping no DSL could express.
func (s *ScriptSender) messageTable(ls *glua.LState, m Message) *glua.LTable {
	tbl := ls.NewTable()

	to := ls.NewTable()
	for _, a := range m.To {
		rec := ls.NewTable()
		rec.RawSetString("email", glua.LString(a.Email))
		rec.RawSetString("name", glua.LString(a.Name))
		to.Append(rec)
	}
	tbl.RawSetString("to", to)

	tbl.RawSetString("subject", glua.LString(m.Subject))
	tbl.RawSetString("html", glua.LString(string(m.HTML)))
	tbl.RawSetString("text", glua.LString(string(m.Text)))

	// The configured envelope sender, so the script does not have to duplicate
	// it in its own config. From is operator config, not message content.
	from := ls.NewTable()
	from.RawSetString("email", glua.LString(s.cfg.From))
	from.RawSetString("name", glua.LString(s.cfg.FromName))
	tbl.RawSetString("from", from)

	// rendered_for is exposed READ-ONLY as information, not as a control: a
	// script cannot widen what it was given, only see whose visibility bounded
	// it. Useful for a provider tag or a log line on the operator's side.
	tbl.RawSetString("rendered_for", glua.LString(m.RenderedFor))

	if len(m.InlineImages) > 0 {
		imgs := ls.NewTable()
		for _, img := range m.InlineImages {
			t := ls.NewTable()
			t.RawSetString("cid", glua.LString(img.CID))
			t.RawSetString("content_type", glua.LString(img.ContentType))
			// Raw bytes. Lua strings are byte strings, so this round-trips,
			// and a script that needs base64 has crypto.base64_encode — which
			// is where that decision belongs, since half the providers want
			// base64 and half want a multipart part.
			t.RawSetString("data", glua.LString(string(img.Data)))
			imgs.Append(t)
		}
		tbl.RawSetString("inline_images", imgs)
	}

	return tbl
}

// resolveScriptPath returns the absolute path to the send script.
//
// Relative paths resolve against the PROJECT ROOT (the parent of .rela), not
// the process working directory: a server's cwd is an accident of how it was
// started, and a transport that delivered mail from one directory and failed
// from another would be a genuinely baffling bug.
func (c *Config) resolveScriptPath() string {
	if filepath.IsAbs(c.Script) {
		return c.Script
	}
	if c.relaDir == "" {
		return c.Script
	}
	return filepath.Join(filepath.Dir(c.relaDir), c.Script)
}

// scriptPathIsContained reports whether a configured script path stays inside
// the project. Used by Validate.
//
// A send script runs with an outbound-HTTP capability and a credential, so
// `script: ../../../etc/something.lua` is worth refusing at load rather than
// discovering at 3am. Absolute paths are also refused for the same reason:
// mail.yaml is project configuration and a project's send script belongs in
// the project.
func scriptPathIsContained(p string) bool {
	if p == "" || filepath.IsAbs(p) {
		return false
	}
	clean := filepath.Clean(p)
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return false
	}
	return true
}

// LuaSender adapts a [Sender] to the interface internal/lua declares for
// mail.send.
//
// The adapter lives HERE, not in internal/lua, because the dependency arrow
// only points one way: lua declares MailSender at its own call site and knows
// nothing about this package, so a lua-side adapter would require an import
// that would make the two packages mutually dependent (and, via ScriptSender,
// literally cyclic).
//
// It wraps a Sender rather than an Outbox on purpose. mail.send from a script
// is SYNCHRONOUS: the script is already running on a worker or a CLI
// invocation, it wants to branch on whether the send worked, and routing it
// through the outbox would hand it a "queued" that means nothing. The outbox
// exists to keep delivery off a user's write path; a script is not that path.
type LuaSender struct {
	sender Sender

	// from is the configured envelope sender. A script does not choose it —
	// the operator does, in mail.yaml — so it is captured here rather than
	// read from the Lua call, where a script could forge it.
	from     string
	fromName string
}

// NewLuaSender adapts sender for use by the mail.send Lua binding.
//
// Nil: a nil sender is rejected rather than wrapped. A LuaSender over nothing
// would report success for mail that was never sent, which is worse than the
// not_configured error the binding produces when no sender is wired at all.
func NewLuaSender(sender Sender, cfg *Config) (*LuaSender, error) {
	if sender == nil {
		return nil, errors.New("mail: nil sender")
	}
	if cfg == nil {
		return nil, errors.New("mail: nil config")
	}
	return &LuaSender{sender: sender, from: cfg.From, fromName: cfg.FromName}, nil
}

// SendMail satisfies lua.MailSender.
//
// RenderedFor is stamped with the send-script principal rather than left
// empty. A script assembles its own body from whatever it can see, so there is
// no per-recipient ACL scoping to record — and an empty RenderedFor would be
// indistinguishable from a declarative render that forgot to set it. Naming
// the system principal says truthfully "no user's visibility bounded this".
func (l *LuaSender) SendMail(ctx context.Context, msg lua.MailMessage) error {
	m := Message{
		Subject:     msg.Subject,
		HTML:        []byte(msg.HTML),
		Text:        []byte(msg.Text),
		RenderedFor: SendScriptPrincipal,
	}
	for _, addr := range msg.To {
		m.To = append(m.To, Address{Email: addr})
	}
	return l.sender.Send(ctx, m)
}

// LoadLuaSender is the [lua.MailSenderLoader] for a project's .rela
// directory: it reads mail.yaml, builds the configured transport, and adapts
// it for the mail.send binding.
//
// Returns (nil, nil) when mail is not configured — the absence is normal and
// mail.send reports it as a typed not_configured error. A mail.yaml that
// EXISTS and is broken returns an error, because "configured but invalid" and
// "not configured" are different facts and only one of them is an operator's
// mistake.
//
// A transport: script project loads a ScriptSender here, which builds its own
// Lua runtime per send. That is not a recursion hazard — the inner runtime has
// no mail sender wired, so a send script calling mail.send gets
// not_configured rather than an unbounded chain of runtimes.
func LoadLuaSender(cacheDir string) (lua.MailSender, error) {
	cfg, err := LoadConfig(cacheDir)
	switch {
	case errors.Is(err, ErrConfigNotFound):
		return nil, nil //nolint:nilnil // "not configured" is a normal absence; see the godoc.
	case err != nil:
		return nil, err
	}

	sender, err := SenderFor(cfg)
	if err != nil {
		return nil, err
	}
	return NewLuaSender(sender, cfg)
}

// SenderFor builds the transport named by cfg.
//
// It lives here rather than at each wiring site because it was previously
// duplicated in internal/appbuild, and a two-copy switch over a closed set is
// how a fourth transport gets wired in one place and not the other. The switch
// is exhaustive; an unknown transport cannot reach here because Validate
// rejects it at load, but the default arm stays as a compile-time reminder
// that a new transport needs a case.
func SenderFor(cfg *Config) (Sender, error) {
	switch cfg.Transport {
	case TransportSMTP:
		return NewSMTPSender(cfg)
	case TransportMemory:
		return NewMemorySender(0), nil
	case TransportHTTP:
		return NewHTTPSender(cfg)
	case TransportScript:
		return NewScriptSender(cfg)
	default:
		return nil, fmt.Errorf("mail: unsupported transport %q", cfg.Transport)
	}
}
