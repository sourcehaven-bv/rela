package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/user"
	"strconv"
	"strings"
	"unicode"

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
// the OS user from $USER. Otherwise runAs must be a job identity
// [validateRunAs] accepts.
//
// A $USER in the `system:` or `integration:` namespace is refused: such a
// name may only enter through `--run-as`, which validates it and records
// asserted_by. Otherwise `USER=system:realtime` would pass for rela's own
// identity with no trace of who set it. See [refuseUser] for the
// look-alikes refused with it.
func newLocalIdentity(tool, runAs string) (localIdentity, error) {
	if runAs == "" {
		name := principal.SystemUser()
		if err := refuseUser(name); err != nil {
			return localIdentity{}, err
		}
		return localIdentity{Principal: principal.Principal{User: name, Tool: tool}}, nil
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

// refuseUser refuses a $USER that names a job identity, or could display
// as one: a `system:` or `integration:` prefix in any letter case, or any
// invisible character (control, format, line or paragraph separator),
// which could hide a prefix or make two names look the same. A letter
// from another script that looks like an ASCII one is not caught; $USER
// is free-form, and only `--run-as` names are held to plain ASCII.
func refuseUser(name string) error {
	if strings.ContainsFunc(name, isInvisible) {
		return fmt.Errorf("$USER %q contains an invisible character; set $USER to your own name", printable(name))
	}
	if principal.IsReserved(strings.ToLower(name)) {
		return fmt.Errorf("$USER %q names a job identity; "+
			"set $USER to your own name and pass the job identity with %s", printable(name), runAsFlag)
	}
	return nil
}

// isInvisible reports a control, format, line or paragraph separator rune.
func isInvisible(r rune) bool {
	return unicode.IsControl(r) || unicode.In(r, unicode.Cf, unicode.Zl, unicode.Zp)
}

// runAsFlag names the flag and its environment variable in error texts.
const runAsFlag = "--run-as (RELA_RUN_AS)"

// validateRunAs accepts a job identity ([principal.ValidJobIdentity]). A
// person's name is refused, so `--run-as` cannot impersonate a human, and
// so is `unknown`.
func validateRunAs(runAs string) error {
	if err := principal.ValidJobIdentity(runAs); err != nil {
		return fmt.Errorf("%s: %w", runAsFlag, err)
	}
	return nil
}

// lookupOSUser returns the account name of the process's uid. A variable
// so a test can simulate a uid with no account.
var lookupOSUser = func() (string, error) { return accountName(os.Getuid()) }

// accountName looks up the account of uid. Where os/user has no system
// lookup, as in the cgo-free Linux release binaries, both user.Current and
// user.LookupId answer the process's own uid from $USER when it is missing
// from /etc/passwd (an LDAP account, a container run with --user). There
// it reads /etc/passwd itself, which has no such fallback.
func accountName(uid int) (string, error) {
	if osUserFallsBackToEnv {
		return passwdName(passwdFile, uid)
	}
	u, err := user.LookupId(strconv.Itoa(uid))
	if err != nil {
		return "", err
	}
	return u.Username, nil
}

// passwdFile is the account database passwdName reads.
const passwdFile = "/etc/passwd"

// passwdName returns the name of uid in the passwd-format file at path.
func passwdName(path string, uid int) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	want := strconv.Itoa(uid)
	for line := range strings.SplitSeq(string(data), "\n") {
		// name:password:uid:gid:gecos:home:shell
		fields := strings.SplitN(line, ":", 4)
		if len(fields) == 4 && fields[2] == want && fields[0] != "" && !strings.HasPrefix(fields[0], "#") {
			return fields[0], nil
		}
	}
	return "", user.UnknownUserIdError(uid)
}

// assertingUser is the OS user behind a `--run-as` identity: the account
// of the process's uid, or `uid:N` when the uid has no account. It is
// looked up, not read from $USER, because the environment is whatever the
// caller set: `USER=alice` would otherwise frame alice.
func assertingUser() string {
	if name, err := lookupOSUser(); err == nil && name != "" {
		return name
	}
	return "uid:" + strconv.Itoa(os.Getuid())
}

// commandIdentity resolves the identity of the matched kong command. A
// command that acts as no one (version, init, completion, bare migrate,
// validate, most of db) ignores a runAs from RELA_RUN_AS, so a value left
// in the environment does not break it, but refuses one given as the flag
// (fromFlag): the operator asked for an identity the command cannot use.
// `rela scheduler` refuses runAs from either source: each task names its
// own identity with `run_as` in schedules.yaml.
func commandIdentity(cmd, runAs string, fromFlag bool) (localIdentity, error) {
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
		if runAs != "" && fromFlag {
			return localIdentity{}, fmt.Errorf("%s does not apply to rela %s: it records no identity",
				runAsFlag, leafCommand(cmd))
		}
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
	if _, ok := identityWrites[leafCommand(cmd)]; ok {
		return true
	}
	return requiresProject(cmd)
}

// identityWrites classifies every command that acts as an identity: true
// when it may write as its principal, so the `--run-as` notice is worth
// printing. TestIdentityWrites_ClassifiesEveryCommand walks the kong tree
// and fails on a command missing here, so a new one is classified on
// purpose. The two db commands are listed because they take no project
// from [requiresProject] yet credit their audited import to the principal.
var identityWrites = map[string]bool{
	"mcp": true, "flow": true, "dev seed": true, "create": true, "update": true, "delete": true,
	"link": true, "unlink": true, "import": true, "fmt": true, "normalize": true,
	"rename entity": true, "rename id": true, "restore": true, "relation-restore": true,
	"token set": true, "token delete": true, "history-tag": true, "history-purge": true,
	"relation-history-purge": true, "attach": true, "detach": true, "gc": true, "script": true,
	"scheduler": true, "renumber": true, "lease claim": true, "lease renew": true, "lease release": true,
	"channel post": true, "channel edit": true, "channel remove": true, "channel create": true,
	"migrate data": true, "migrate gc": true, "migrate adopt-face": true, "migrate baseline": true,
	"db load": true, "db import-fs": true,

	"migrate status": false, "migrate gen": false, "show": false, "list": false,
	"trace from": false, "trace to": false, "trace path": false, "graph": false, "export": false,
	"render": false, "template init": false, "schema overview": false, "schema entities": false,
	"schema relations": false, "schema types": false, "schema entity": false, "schema relation": false,
	"analyze orphans": false, "analyze duplicates": false, "analyze unique": false, "analyze gaps": false,
	"analyze cardinality": false, "analyze relation-order": false, "analyze relation-files": false,
	"analyze owning": false, "analyze properties": false, "analyze validations": false,
	"analyze states": false, "analyze all": false, "analyze schema": false, "acl audit": false,
	"acl who-can": false, "acl can": false, "acl can-relation": false, "acl map": false,
	"history": false, "relation-history": false, "token status": false, "attachments": false,
	"secrets credential-name": false, "lease list": false, "channel list": false, "channel read": false,
	"channel watch": false,
}

// writesAsIdentity reports whether cmd may write as its principal.
func writesAsIdentity(cmd string) bool {
	return identityWrites[leafCommand(cmd)]
}

// leafCommand reduces kong's Command() string to the command names, without
// the positional placeholders: "trace from <id>" gives "trace from".
func leafCommand(cmd string) string {
	var names []string
	for f := range strings.FieldsSeq(cmd) {
		if strings.HasPrefix(f, "<") || strings.HasPrefix(f, "[") {
			break
		}
		names = append(names, f)
	}
	return strings.Join(names, " ")
}
