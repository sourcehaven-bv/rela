package appbuild

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/ai"
	"github.com/Sourcehaven-BV/rela/internal/flowengine"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/mail"
)

// seams are the determinism options: what a world reads from the clock, the
// network and the disk, replaced. Each field is nil (or empty) when the
// caller did not set it, and the build then behaves as before.
type seams struct {
	clock     func() time.Time
	mail      mail.Sender
	http      *http.Client
	ai        ai.Provider
	outputDir string
	flowJobs  flowengine.JobQueue
}

// WithClock sets the clock of the world. It reaches:
//   - `{{now}}` and `{{today}}` in automation, copy and conversion templates;
//   - today() in every predicate: automation and validation conditions,
//     `enforce: write` rules, and query scopes resolved through
//     [SharedBase.QueryScopes] (the package-level [QueryScopes] stays on the
//     wall clock);
//   - computed properties;
//   - rela.today, rela.now and the date helpers in every Lua runtime built
//     from these services (automations, actions, scheduled tasks, validation
//     rules, scripts);
//   - the in-memory store's UpdatedAt and soft-delete stamps ([NewInMemory]),
//     and the soft-delete purge that ages them;
//   - lease expiry;
//   - the "now" of a scheduled mail template.
//
// Audit records and version history stay on the wall clock. They record when
// a write really happened, and a test that pins the clock must not be able to
// backdate them.
//
// Only runtimes and evaluators built by these services see it. A host that
// builds its own (the data-entry app's action and validation runtimes) does
// not.
//
// Nil: ignored — the wall clock.
func WithClock(now func() time.Time) Option {
	return func(o *options) {
		if now != nil {
			o.seams.clock = now
		}
	}
}

// WithMailTransport replaces the transport mail.yaml names with s. The file
// is still read: the sender address, the base URL and the recipients
// allowlist still come from it, and without a mail.yaml mail stays off. Both
// mail.send in Lua and scheduled mail templates deliver through s.
//
// Nil: ignored — mail.yaml decides.
func WithMailTransport(s mail.Sender) Option {
	return func(o *options) {
		if s != nil {
			o.seams.mail = s
		}
	}
}

// WithHTTPClient sets the client the Lua http.* bindings send through in
// every runtime built from these services.
//
// Nil: ignored — the shared process-wide client.
func WithHTTPClient(c *http.Client) Option {
	return func(o *options) {
		if c != nil {
			o.seams.http = c
		}
	}
}

// WithAIProvider sets the provider the Lua ai.* bindings use in every writer
// runtime built from these services. It wins over ai.yaml, which is then not
// read.
//
// Nil: ignored — ai.yaml decides.
func WithAIProvider(p ai.Provider) Option {
	return func(o *options) {
		if p != nil {
			o.seams.ai = p
		}
	}
}

// WithOutputDir sets the directory rela.write_file writes to in every
// runtime built from these services. It wins over a per-call output
// directory, such as the --output flag of `rela script`. An absolute path is
// used as is; a relative one is joined to the project root.
//
// Empty: ignored — output/ under the project root.
func WithOutputDir(dir string) Option {
	return func(o *options) {
		if dir != "" {
			o.seams.outputDir = dir
		}
	}
}

// WithFlowJobQueue makes the flow engine put job step attempts on q instead
// of the job queue, in every process, long-running or not. The caller runs
// each attempt with [flowengine.AttemptHandler.HandleAttempt] over
// [FlowEngine], when it chooses. `rela test` uses it to run attempts in the
// case, one by one, on the case's clock.
//
// Nil: ignored — the job queue of a long-running process, else none.
func WithFlowJobQueue(q flowengine.JobQueue) Option {
	return func(o *options) {
		if q != nil {
			o.seams.flowJobs = q
		}
	}
}

// clock returns the world's clock: the [WithClock] one, else the wall clock.
func (b *SharedBase) clock() func() time.Time {
	if b.opts.seams.clock != nil {
		return b.opts.seams.clock
	}
	return time.Now
}

// luaSeams returns the seams every Lua runtime built from these services
// carries.
func (b *SharedBase) luaSeams() lua.Seams {
	s := b.opts.seams
	return lua.Seams{
		Clock:      s.clock,
		HTTPClient: s.http,
		AI:         s.ai,
		Mail:       luaMailLoader(s.mail),
		OutputDir:  s.outputDir,
	}
}

// luaMailLoader returns the mail.send loader over transport: mail.yaml is
// parsed as usual, so the sender address, base URL and recipients policy
// apply, and only the transport is replaced. Nil when transport is nil, so
// mail.yaml's own transport is used.
func luaMailLoader(transport mail.Sender) lua.MailSenderLoader {
	if transport == nil {
		return nil
	}
	return func(host lua.HostConfig) (lua.MailSender, error) {
		cfg, err := mail.LoadConfigFrom(host)
		switch {
		case errors.Is(err, mail.ErrConfigNotFound):
			return nil, nil // mail is not configured; see lua.MailSenderLoader.
		case err != nil:
			return nil, err
		}
		return mail.NewLuaSender(transport, cfg)
	}
}

// leaseClock adapts a clock function to [lease.Clock].
type leaseClock func() time.Time

// Now returns the time.
func (c leaseClock) Now() time.Time { return c() }

// WithPolicyAmendment lets amend edit the policy parsed from acl.yaml before
// it is validated against the schema and used. The `rela test` runner uses it
// to assign roles to the principals a case declares.
//
// Every build parses acl.yaml anew, so amend edits a policy no other
// [Services] holds; the file and any other build's policy are untouched. An
// error from amend fails the build.
//
// It is ignored when there is no acl.yaml (the build then allows every
// access) and when [WithACL] supplies the ACL.
//
// Nil: ignored — the policy is used as parsed.
func WithPolicyAmendment(amend func(*acl.Policy) error) Option {
	return func(o *options) {
		if amend != nil {
			o.policyAmendment = amend
		}
	}
}

// amendPolicy applies amend to policy. A nil policy (no acl.yaml) or amend
// is left alone.
func amendPolicy(policy *acl.Policy, amend func(*acl.Policy) error) error {
	if policy == nil || amend == nil {
		return nil
	}
	if err := amend(policy); err != nil {
		return fmt.Errorf("appbuild: amend acl.yaml: %w", err)
	}
	return nil
}
