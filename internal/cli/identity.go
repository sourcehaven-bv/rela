package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// localIdentity is who a local CLI or stdio `rela mcp` process acts as.
//
// Without `--run-as` it is the OS user ($USER) with the transport's tool,
// as it always was. With `--run-as` it is the named job identity, and
// AssertedBy is the OS user who asserted it. The name is attribution, not
// authentication: a local user has full access to the project anyway. It
// carries no claims (no principal type, no roles), so acl.yaml decides
// what it may do through its assignments alone.
type localIdentity struct {
	Principal principal.Principal
	// AssertedBy is the OS user behind a `--run-as` identity; empty without
	// one.
	AssertedBy string
}

// newLocalIdentity resolves the identity for tool. An empty runAs gives
// the OS user. Otherwise runAs must be a job identity [validateRunAs]
// accepts.
func newLocalIdentity(tool, runAs string) (localIdentity, error) {
	if runAs == "" {
		return localIdentity{Principal: principal.Principal{User: principal.SystemUser(), Tool: tool}}, nil
	}
	if err := validateRunAs(runAs); err != nil {
		return localIdentity{}, err
	}
	return localIdentity{
		Principal:  principal.Principal{User: runAs, Tool: tool},
		AssertedBy: assertingUser(),
	}, nil
}

// stamp puts the identity on ctx: the principal, and the asserting OS user
// for the graph audit when the identity was asserted with `--run-as`.
func (l localIdentity) stamp(ctx context.Context) context.Context {
	ctx = principal.With(ctx, l.Principal)
	if l.AssertedBy != "" {
		ctx = audit.WithAssertedBy(ctx, l.AssertedBy)
	}
	return ctx
}

// notice prints "acting as X (asserted by Y)" to w when the identity was
// asserted with `--run-as`, so a person running a write command sees whom
// it is attributed to.
func (l localIdentity) notice(w io.Writer) {
	if l.AssertedBy == "" {
		return
	}
	_, _ = fmt.Fprintf(w, "acting as %s (asserted by %s)\n", printable(l.Principal.User), printable(l.AssertedBy))
}

// runAsFlag names the flag and its environment variable in error texts.
const runAsFlag = "--run-as (RELA_RUN_AS)"

// refusedRunAs are rela's own identities. Each one belongs to an
// in-process entry point whose writes people trust as rela's: a trigger
// post signed system:realtime, a scheduled task, a background automation,
// a provisioned user. `--run-as` must not borrow them.
var refusedRunAs = map[string]bool{
	principal.UserRealtime:    true,
	principal.UserScheduler:   true,
	principal.UserAutomation:  true,
	principal.UserProvisioner: true,
}

// webhookUserPrefix names the identities of declarative webhook deliveries
// (see [principal.Webhook]), which are rela's own too.
const webhookUserPrefix = principal.ReservedPrefix + "webhook:"

// validateRunAs accepts a job identity: a `system:` name that is not one
// of rela's own, or an `integration:` name. A person's name is refused, so
// `--run-as` cannot impersonate a human, and so is `unknown`.
func validateRunAs(runAs string) error {
	if !strings.HasPrefix(runAs, principal.ReservedPrefix) && !strings.HasPrefix(runAs, principal.IntegrationPrefix) {
		return fmt.Errorf("%s: %q is not a job identity; name a %q or %q identity",
			runAsFlag, runAs, principal.ReservedPrefix, principal.IntegrationPrefix)
	}
	if err := principal.ValidateRunAs(runAs, ownJobIdentity); err != nil {
		return fmt.Errorf("%s: %w", runAsFlag, err)
	}
	return nil
}

// ownJobIdentity reports whether a `system:` name is free for `--run-as`.
func ownJobIdentity(name string) bool {
	return name != principal.ReservedPrefix && !refusedRunAs[name] && !strings.HasPrefix(name, webhookUserPrefix)
}

// assertingUser is the OS user behind a `--run-as` identity: $USER (or
// $USERNAME), then $LOGNAME, then the numeric uid. A cron or systemd job
// often has no $USER, and the audit still needs someone to name.
func assertingUser() string {
	if u := principal.SystemUser(); u != principal.Unknown {
		return u
	}
	if u := strings.TrimSpace(os.Getenv("LOGNAME")); u != "" {
		return u
	}
	return "uid:" + strconv.Itoa(os.Getuid())
}

// commandIdentity resolves the identity of the matched kong command. A
// command that acts as no one (version, init, completion, bare migrate,
// validate) ignores runAs, so a RELA_RUN_AS left in the environment does
// not break it. `rela scheduler` refuses runAs: each task names its own
// identity with `run_as` in schedules.yaml.
func commandIdentity(cmd, runAs string) (localIdentity, error) {
	first := firstKongToken(cmd)
	if first == "scheduler" && runAs != "" {
		return localIdentity{}, fmt.Errorf("%s does not apply to rela scheduler: "+
			"each task names its identity with run_as in schedules.yaml", runAsFlag)
	}
	tool := principal.ToolCLI
	if first == "mcp" {
		tool = principal.ToolMCP
	}
	if !actsAsIdentity(cmd) {
		runAs = ""
	}
	return newLocalIdentity(tool, runAs)
}

// actsAsIdentity reports whether cmd reads or writes the project as a
// principal.
func actsAsIdentity(cmd string) bool {
	switch firstKongToken(cmd) {
	case "mcp", "flow":
		return true
	}
	return requiresProject(cmd)
}

// readOnlyCommands write nothing to the project, so they print no
// `--run-as` notice.
var readOnlyCommands = map[string]bool{
	"show": true, "list": true, "trace": true, "graph": true, "export": true, "render": true,
	"schema": true, "analyze": true, "acl": true, "attachments": true, "history": true,
	"relation-history": true, "secrets": true,
	"channel list": true, "channel read": true, "channel watch": true, "lease list": true,
}

// writesAsIdentity reports whether cmd may write as its principal, so the
// `--run-as` notice is worth printing.
func writesAsIdentity(cmd string) bool {
	return actsAsIdentity(cmd) && !readOnlyCommands[firstKongToken(cmd)] && !readOnlyCommands[firstTwoKongTokens(cmd)]
}
