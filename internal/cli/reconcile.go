package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	relaerrors "github.com/Sourcehaven-BV/rela/internal/errors"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/output"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/reconclient"
	"github.com/Sourcehaven-BV/rela/internal/reconclient/httpremote"
	"github.com/Sourcehaven-BV/rela/internal/secrets"
)

// ReconcileCmd groups the commands that compare this project with another
// rela over POST /api/v1/_reconcile (FEAT-CMVZQFI).
type ReconcileCmd struct {
	Diff ReconcileDiffCmd `cmd:"" help:"Compare the entities and relations of this project with a remote rela."`
}

// reconcileServices is the binding `rela reconcile` takes: the local gated
// element read, built on first use, and the schema the scope is checked
// against.
type reconcileServices struct {
	// Local builds the local element read.
	//
	// Nil: accepted. The command then fails, as on a project that cannot
	// build the read.
	Local func() (reconclient.Local, error)
	Meta  *metamodel.Metamodel
	// Secret reads a value from the global section of the project's secrets
	// (`.rela/secrets.yaml` or the systemd credential), for --token-secret.
	//
	// Nil: accepted. --token-secret then fails.
	Secret func(name string) (string, error)
}

// ReconcileDiffCmd compares this project's view with a remote's view.
//
// The credential comes from an environment variable, a file or a project
// secret, never from a flag value, so it does not show up in ps or in shell
// history. Each is named explicitly: a credential is never picked up from a
// default and sent to whatever --remote names.
type ReconcileDiffCmd struct {
	Remote            string        `required:"" help:"Base URL of the remote rela, such as https://rela.example.com."`
	Types             []string      `sep:"," help:"Entity and relation types to compare (default: every type in the schema). A relation type needs the entity types at both of its ends."`
	TokenEnv          string        `name:"token-env" xor:"credential" help:"Environment variable that holds the credential for the remote, such as RELA_RECONCILE_TOKEN."`
	TokenFile         string        `name:"token-file" type:"path" xor:"credential" help:"File that holds the credential for the remote."`
	TokenSecret       string        `name:"token-secret" xor:"credential" help:"Name of the global project secret (.rela/secrets.yaml or its systemd credential) that holds the credential for the remote. The name starts with 'remote_credential.', for example 'remote_credential.upstream'."`
	AuthHeader        string        `name:"auth-header" default:"Authorization" help:"Request header that carries the credential. Authorization sends it as a bearer token."`
	AllowInsecureHTTP bool          `name:"allow-insecure-http" help:"Allow plain http to a host that is not loopback."`
	Timeout           time.Duration `default:"10m" help:"Time limit for the whole comparison."`
	MaxRoundTrips     int           `name:"max-round-trips" default:"1000" help:"Maximum number of requests to the remote."`
	ExitCode          bool          `name:"exit-code" help:"Exit with 1 when the two sides differ and 2 on an error."`
}

// maxTokenFile caps the credential file read by --token-file.
const maxTokenFile = 16 << 10

// Run compares the two sides and prints the differences.
func (c *ReconcileDiffCmd) Run(ctx context.Context, s *reconcileServices) error {
	err := c.run(ctx, s, os.Getenv)
	if err != nil && c.ExitCode {
		var exit *relaerrors.ExitError
		if !errors.As(err, &exit) {
			fmt.Fprintln(os.Stderr, err)
			return relaerrors.NewExitError(2)
		}
	}
	return err
}

func (c *ReconcileDiffCmd) run(ctx context.Context, s *reconcileServices, getenv func(string) string) error {
	if s == nil || s.Local == nil {
		return errors.New("reconcile: this project cannot read its element ranges")
	}
	if c.MaxRoundTrips < 1 {
		return errors.New("--max-round-trips must be at least 1")
	}
	if c.Timeout <= 0 {
		return errors.New("--timeout must be positive")
	}
	scope, err := reconcileScope(s.Meta, c.Types)
	if err != nil {
		return err
	}
	cfg := httpremote.Config{BaseURL: c.Remote, AllowInsecureHTTP: c.AllowInsecureHTTP}
	value, source, err := c.credential(getenv, s.Secret)
	if err != nil {
		return err
	}
	if value != "" {
		if cfg.Auth, err = httpremote.Header(c.AuthHeader, value); err != nil {
			// The error names the rule, never the value.
			return fmt.Errorf("%s: %w", source, err)
		}
	}
	remote, err := httpremote.New(cfg)
	if err != nil {
		return err
	}
	local, err := s.Local()
	if err != nil {
		return fmt.Errorf("reconcile: %w", err)
	}
	d, err := reconclient.New(local, remote, reconclient.Options{
		MaxRoundTrips: c.MaxRoundTrips, Deadline: c.Timeout,
	})
	if err != nil {
		return err
	}
	res, err := d.Diff(ctx, scope)
	if err != nil {
		if errors.Is(err, acl.ErrNoIdentity) {
			return fmt.Errorf("%w (the local side reads as your rela identity; see the warning above, or set --run-as)", err)
		}
		return err
	}
	who := principal.From(ctx).User
	if out.Format == output.FormatJSON {
		err = writeReconcileJSON(out.Out, remote.Origin(), who, res)
	} else {
		err = writeReconcileText(out.Out, remote.Origin(), who, res)
	}
	if err != nil {
		return err
	}
	if c.ExitCode && !res.Same() {
		return relaerrors.NewExitError(1)
	}
	return nil
}

// credential reads the credential from --token-secret, --token-file or the
// variable --token-env names, and names its source for errors. No flag means
// no credential; a named source that is empty is an error.
func (c *ReconcileDiffCmd) credential(getenv func(string) string,
	secret func(string) (string, error),
) (value, source string, err error) {
	switch {
	case c.TokenSecret != "":
		source = "--token-secret " + c.TokenSecret
		if !secrets.IsRemoteCredential(c.TokenSecret) {
			// Only a name no script can be granted; see
			// [secrets.RemoteCredentialPrefix].
			return "", "", fmt.Errorf("%s: the name must start with %q followed by a name, "+
				"so that no script can read the secret", source, secrets.RemoteCredentialPrefix)
		}
		if secret == nil {
			return "", "", fmt.Errorf("%s: this project has no secrets", source)
		}
		v, sErr := secret(c.TokenSecret)
		if sErr != nil {
			return "", "", fmt.Errorf("%s: %w", source, sErr)
		}
		value = strings.TrimSpace(v)
	case c.TokenFile != "":
		if value, err = readTokenFile(c.TokenFile); err != nil {
			return "", "", err
		}
		source = "--token-file"
	case c.TokenEnv != "":
		value, source = strings.TrimSpace(getenv(c.TokenEnv)), "$"+c.TokenEnv
	default:
		return "", "", nil
	}
	if value == "" {
		return "", "", fmt.Errorf("%s: no credential (empty or unset)", source)
	}
	return value, source, nil
}

// readTokenFile reads a credential file of at most maxTokenFile bytes and
// warns when other users can read it.
func readTokenFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("--token-file: %w", err)
	}
	defer f.Close()
	if st, sErr := f.Stat(); sErr == nil && st.Mode().Perm()&0o077 != 0 {
		slog.Warn("reconcile: the credential file can be read by other users", "file", path,
			"mode", st.Mode().Perm().String())
	}
	b, err := io.ReadAll(io.LimitReader(f, maxTokenFile+1))
	if err != nil {
		return "", fmt.Errorf("--token-file: %w", err)
	}
	if len(b) > maxTokenFile {
		return "", fmt.Errorf("--token-file: the file is larger than %d bytes", maxTokenFile)
	}
	return strings.TrimSpace(string(b)), nil
}

// reconcileScope turns --types into a scope checked against the local
// schema. No types means every type. A relation type whose declared ends
// are all outside the entity scope could never be compared, so it is an
// error rather than a silent empty result.
func reconcileScope(meta *metamodel.Metamodel, types []string) (reconclient.Scope, error) {
	if meta == nil {
		return reconclient.Scope{}, errors.New("reconcile: the project has no schema")
	}
	var scope reconclient.Scope
	if len(types) == 0 {
		scope.Entities = meta.EntityTypes()
		scope.Relations = meta.RelationTypes()
	} else {
		for _, t := range types {
			t = strings.TrimSpace(t)
			switch {
			case t == "":
				continue
			case meta.HasEntityType(t):
				scope.Entities = append(scope.Entities, t)
			default:
				if _, ok := meta.GetRelationDef(t); !ok {
					return reconclient.Scope{}, fmt.Errorf("--types: %q is not an entity or relation type in the schema",
						t)
				}
				scope.Relations = append(scope.Relations, t)
			}
		}
		for _, r := range scope.Relations {
			def, _ := meta.GetRelationDef(r)
			for _, end := range []struct {
				name  string
				types []string
			}{{"from", def.From}, {"to", def.To}} {
				inScope := slices.ContainsFunc(end.types, func(t string) bool {
					return slices.Contains(scope.Entities, t)
				})
				if !inScope {
					return reconclient.Scope{}, fmt.Errorf(
						"--types: relation type %q needs an entity type at its %s end in scope (one of %s)",
						r, end.name, strings.Join(end.types, ", "))
				}
			}
		}
	}
	slices.Sort(scope.Entities)
	scope.Entities = slices.Compact(scope.Entities)
	slices.Sort(scope.Relations)
	scope.Relations = slices.Compact(scope.Relations)
	return scope, nil
}

// reconcileJSON is the `rela reconcile diff -o json` answer.
type reconcileJSON struct {
	Remote     string                 `json:"remote"`
	Principal  string                 `json:"principal"`
	Same       bool                   `json:"same"`
	OnlyLocal  []reconcileElementJSON `json:"only_local"`
	OnlyRemote []reconcileElementJSON `json:"only_remote"`
	Differing  []reconcileChangeJSON  `json:"differing"`
	Skipped    []reconcileSkipJSON    `json:"skipped"`
	RoundTrips int                    `json:"round_trips"`
}

type reconcileElementJSON struct {
	Kind    string `json:"kind"`
	Address string `json:"address"`
	Hash    string `json:"hash"`
}

type reconcileChangeJSON struct {
	Kind       string `json:"kind"`
	Address    string `json:"address"`
	LocalHash  string `json:"local_hash"`
	RemoteHash string `json:"remote_hash"`
}

type reconcileSkipJSON struct {
	Kind string `json:"kind"`
	Type string `json:"type"`
	// RefusedBy is "local" or "remote": whose policy left the type out.
	RefusedBy string `json:"refused_by"`
	Reason    string `json:"reason"`
}

// refusedBy names whose policy left s out.
func refusedBy(s reconclient.Skipped) string {
	if s.Remote {
		return "remote"
	}
	return "local"
}

func reconcileElements(els []reconclient.Element) []reconcileElementJSON {
	rows := make([]reconcileElementJSON, 0, len(els))
	for _, e := range els {
		rows = append(rows, reconcileElementJSON{Kind: e.Kind.String(), Address: e.Address, Hash: e.Hash.String()})
	}
	return rows
}

func writeReconcileJSON(w io.Writer, origin, who string, res reconclient.Result) error {
	doc := reconcileJSON{
		Remote: origin, Principal: who, Same: res.Same(),
		OnlyLocal: reconcileElements(res.OnlyLocal), OnlyRemote: reconcileElements(res.OnlyRemote),
		Differing:  make([]reconcileChangeJSON, 0, len(res.Differing)),
		Skipped:    make([]reconcileSkipJSON, 0, len(res.Skipped)),
		RoundTrips: res.Stats.RoundTrips,
	}
	for _, c := range res.Differing {
		doc.Differing = append(doc.Differing, reconcileChangeJSON{
			Kind: c.Kind.String(), Address: c.Address, LocalHash: c.Local.String(), RemoteHash: c.Remote.String(),
		})
	}
	for _, s := range res.Skipped {
		doc.Skipped = append(doc.Skipped, reconcileSkipJSON{
			Kind: s.Kind.String(), Type: s.Type, RefusedBy: refusedBy(s), Reason: s.Reason,
		})
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(doc)
}

// shown returns s as is when it is safe to print, and quoted otherwise, so
// a stored address cannot drive the terminal.
func shown(s string) string {
	if reconclient.Printable(s) && !strings.ContainsAny(s, " \t") {
		return s
	}
	return strconv.Quote(s)
}

func writeReconcileText(w io.Writer, origin, who string, res reconclient.Result) error {
	var b strings.Builder
	if who == "" {
		who = "(no identity)"
	}
	fmt.Fprintf(&b, "Compared the view of %s here with the view the credential has on %s.\n", shown(who), origin)
	section := func(title string, n int) {
		fmt.Fprintf(&b, "\n%s (%d)\n", title, n)
	}
	section("Only here", len(res.OnlyLocal))
	for _, e := range res.OnlyLocal {
		fmt.Fprintf(&b, "  + %s\n", shown(e.Address))
	}
	section("Only on the remote", len(res.OnlyRemote))
	for _, e := range res.OnlyRemote {
		fmt.Fprintf(&b, "  - %s\n", shown(e.Address))
	}
	section("Different", len(res.Differing))
	for _, c := range res.Differing {
		fmt.Fprintf(&b, "  ~ %s\n", shown(c.Address))
	}
	if len(res.Skipped) > 0 {
		section("Not compared", len(res.Skipped))
		for _, s := range res.Skipped {
			fmt.Fprintf(&b, "  %s %s (%s policy): %s\n", s.Kind.String(), shown(s.Type), refusedBy(s),
				reconclient.Sanitize(s.Reason))
		}
	}
	if res.Same() {
		b.WriteString("\nNo differences.\n")
	}
	fmt.Fprintf(&b, "\n%d requests to the remote.\n", res.Stats.RoundTrips)
	_, err := io.WriteString(w, b.String())
	return err
}
