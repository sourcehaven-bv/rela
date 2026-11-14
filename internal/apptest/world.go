package apptest

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"math/rand/v2"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/attachment"
	"github.com/Sourcehaven-BV/rela/internal/comments/memcomments"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/fsimport"
	"github.com/Sourcehaven-BV/rela/internal/mail"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// world is one case's in-memory app. It is built on first use ([world.ensure])
// so that the case's setup calls can shape it first.
type world struct {
	r    *runner
	seed uint64
	// ctx is the running case's context, with its timeout. The Lua calls
	// that write and read take no context, so the world carries it.
	ctx context.Context //nolint:containedctx // see above

	// clock is the case's clock in Unix nanoseconds. Atomic, because the
	// world's background workers read it while the case sets it.
	clock atomic.Int64
	// network is set when the case's code tried a real HTTP or AI call.
	network atomic.Bool
	rng     *rand.Rand
	fake    *faker

	// Setup recorded before the world is built.
	fixtures   []string
	principals []*testPrincipal

	svc  *appbuild.Services
	bind func(context.Context) (context.Context, error)
	mail *mail.MemorySender
	// buildErr is the error the first build failed with; later calls
	// return it again rather than retrying.
	buildErr error

	// failure is the first failed assertion of the case.
	failure *failure
	// lastErr is the Go error of the last failed write or read, for
	// t.expect_denied.
	lastErr error
}

// failure is a failed assertion: where it was and why.
type failure struct {
	where string
	msg   string
}

// testPrincipal is a principal declared with t.principal.
type testPrincipal struct {
	name  string
	roles []string
	// userID is the id of its user entity, or "" when acl.yaml names no
	// user entity type. Without principal_property it is the name itself.
	userID string
	// assignee is the key its first role is assigned under: the user
	// entity's id when principal_property maps the name to one, else the
	// name. It is the id the identity resolver walks membership from.
	assignee string
	// groups are the ids of the group entities that carry roles[1:].
	groups []string
}

// idSlug turns a principal name into an entity id fragment: every character
// the id grammar does not allow becomes "-", and a leading one becomes "p".
func idSlug(name string) string {
	b := []byte(name)
	for i, c := range b {
		ok := c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '-'
		if !ok {
			b[i] = '-'
		}
	}
	s := string(b)
	if s == "" || s[0] == '_' || s[0] == '-' {
		s = "p" + s
	}
	return s
}

func newWorld(r *runner, seed uint64) *world {
	w := &world{r: r, seed: seed, ctx: context.Background(), rng: seededRand(seed), mail: mail.NewMemorySender(0)}
	w.clock.Store(startClock(seed).UnixNano())
	// The faker exists before the world so t.fake.name and the like work in
	// setup; build gives it the schema.
	w.fake = newFaker(seed, w.now, nil)
	return w
}

// now is the case's clock.
func (w *world) now() time.Time { return time.Unix(0, w.clock.Load()).UTC() }

// built reports whether the world exists yet.
func (w *world) built() bool { return w.svc != nil || w.buildErr != nil }

// ensure builds the world on first use: the in-memory services, the
// fixtures, and the declared principals' user entities.
func (w *world) ensure() error {
	if w.svc != nil || w.buildErr != nil {
		return w.buildErr
	}
	w.buildErr = w.build()
	return w.buildErr
}

func (w *world) build() error {
	svc, err := appbuild.NewInMemory(w.r.root,
		appbuild.WithClock(w.now),
		appbuild.WithMailTransport(w.mail),
		appbuild.WithPolicyAmendment(w.assignRoles),
		appbuild.WithHTTPClient(&http.Client{Transport: refusingTransport{tried: &w.network}}),
		appbuild.WithAIProvider(refusingAI{tried: &w.network}),
	)
	if err != nil {
		return fmt.Errorf("build the test world: %w", err)
	}
	w.svc = svc
	w.fake.meta = svc.Meta()
	if w.bind, err = appbuild.IdentityBinder(svc); err != nil {
		return err
	}
	for _, dir := range w.fixtures {
		if err := w.loadFixture(dir); err != nil {
			return err
		}
	}
	for _, p := range w.principals {
		if err := w.createPrincipal(p); err != nil {
			return err
		}
	}
	return nil
}

// close releases the world.
func (w *world) close() error {
	if w.svc == nil {
		return nil
	}
	return w.svc.Close()
}

// assignRoles is the policy amendment of the case's world: each declared
// principal's first role is assigned to it, and each further role to one of
// its group entities. Only this world's copy of the policy changes.
func (w *world) assignRoles(p *acl.Policy) error {
	for _, tp := range w.principals {
		if len(tp.roles) == 0 {
			continue
		}
		if p.Assignments == nil {
			p.Assignments = map[string]string{}
		}
		if prev, taken := p.Assignments[tp.assignee]; taken {
			return fmt.Errorf("t.principal: %s is already assigned role %s in acl.yaml", tp.assignee, prev)
		}
		p.Assignments[tp.assignee] = tp.roles[0]
		for i, g := range tp.groups {
			if prev, taken := p.Assignments[g]; taken {
				return fmt.Errorf("t.principal: group %s of %s is already assigned role %s in acl.yaml", g, tp.name, prev)
			}
			p.Assignments[g] = tp.roles[i+1]
		}
	}
	return nil
}

// declarePrincipal records a principal for the world to create. It fails
// once the world exists, and for a role acl.yaml does not declare.
func (w *world) declarePrincipal(name string, roles []string) (*testPrincipal, error) {
	if w.built() {
		return nil, errors.New("t.principal must come before the case's first write or read; " +
			"declare principals at the start of the case")
	}
	pol := w.r.policy
	if len(roles) > 0 && pol == nil {
		return nil, errors.New("t.principal: the project has no acl.yaml, so there are no roles to give; " +
			"without acl.yaml every principal may do everything")
	}
	for _, role := range roles {
		if role == acl.EveryoneRole {
			return nil, fmt.Errorf("t.principal: every principal already holds the %s role", acl.EveryoneRole)
		}
		if _, ok := pol.Roles[role]; !ok {
			return nil, fmt.Errorf("t.principal: acl.yaml declares no role %q", role)
		}
	}
	if len(roles) > 1 && strings.TrimSpace(pol.UserEntityType) == "" {
		return nil, errors.New("t.principal: more than one role needs user_entity_type in acl.yaml, " +
			"because the further roles are given through group entities")
	}
	for _, p := range w.principals {
		if p.name == name {
			return nil, fmt.Errorf("t.principal: %s is already declared in this case", name)
		}
	}
	tp := &testPrincipal{name: name, roles: roles, assignee: name}
	if pol != nil && strings.TrimSpace(pol.UserEntityType) != "" {
		tp.userID = name
		if strings.TrimSpace(pol.PrincipalProperty) != "" {
			// The name is a natural key (often an email); the resolver maps
			// it to the user entity, and grants are looked up by its id.
			tp.userID = "user-" + idSlug(name)
			tp.assignee = tp.userID
		} else if err := entity.ValidateID(name); err != nil {
			return nil, fmt.Errorf("t.principal: %s is the id of its %s entity, and %w; "+
				"pick a name the id grammar allows, or set principal_property in acl.yaml",
				name, pol.UserEntityType, err)
		}
	}
	for i := 1; i < len(roles); i++ {
		tp.groups = append(tp.groups, fmt.Sprintf("%s-group-%d", idSlug(name), i))
	}
	w.principals = append(w.principals, tp)
	return tp, nil
}

// createPrincipal writes the principal's user entity and group entities
// as given state: straight to the store, with no ACL, automations or
// validation. Nothing is written when acl.yaml names no user entity type.
func (w *world) createPrincipal(tp *testPrincipal) error {
	pol := w.svc.ACLPolicy()
	if pol == nil || strings.TrimSpace(pol.UserEntityType) == "" {
		return nil
	}
	userType := strings.TrimSpace(pol.UserEntityType)
	if _, ok := w.svc.Meta().Entities[userType]; !ok {
		return fmt.Errorf("t.principal: acl.yaml names user_entity_type %s, which the schema does not declare", userType)
	}
	ctx := w.systemContext()
	props := map[string]any{}
	if prop := strings.TrimSpace(pol.PrincipalProperty); prop != "" {
		props[prop] = tp.name
	}
	user, err := w.givenEntity(ctx, userType, tp.userID, props)
	if err != nil {
		return fmt.Errorf("t.principal %s: %w", tp.name, err)
	}
	rel := pol.EffectiveMembershipRelation()
	for _, g := range tp.groups {
		if _, err := w.givenEntity(ctx, userType, g, map[string]any{}); err != nil {
			return fmt.Errorf("t.principal %s: group %s: %w", tp.name, g, err)
		}
		key := entity.RelationKey{From: user.ID, FromFace: user.Face, Type: rel, To: g}
		if _, err := w.svc.Store().CreateRelation(ctx, key, nil); err != nil {
			return fmt.Errorf("t.principal %s: link group %s: %w", tp.name, g, err)
		}
	}
	return nil
}

// givenEntity writes the entity typ/id straight to the store, with its
// required properties generated.
func (w *world) givenEntity(ctx context.Context, typ, id string, props map[string]any) (*entity.Entity, error) {
	if _, err := w.fake.fill(typ, props); err != nil {
		return nil, err
	}
	e := &entity.Entity{ID: id, Type: typ, Properties: props}
	if faces := metamodel.FaceOrderOf(w.svc.Meta(), typ); len(faces) > 0 {
		e.Face = entity.Face(faces[0])
	}
	if err := w.svc.Store().CreateEntity(ctx, e); err != nil {
		return nil, err
	}
	return e, nil
}

// systemContext is the context of a given-state write: attributed to the
// test runner, with no identity bound.
func (w *world) systemContext() context.Context {
	return principal.With(w.ctx, principal.Principal{User: principal.UserFSImport, Tool: principal.ToolRelaTest})
}

// noSymlinks fails when a component of rel, a local path under root, is a
// symlink or does not exist.
func noSymlinks(root, rel string) error {
	cur := root
	for part := range strings.SplitSeq(rel, string(filepath.Separator)) {
		cur = filepath.Join(cur, part)
		fi, err := os.Lstat(cur)
		if err != nil {
			return err
		}
		if fi.Mode()&fs.ModeSymlink != 0 {
			return fmt.Errorf("%s is a symlink", cur)
		}
	}
	return nil
}

// loadFixture copies the entity and relation markdown under dir into the
// store as given state, through [fsimport.Copy]. Comments under dir are not
// loaded: they go to a store the world does not read, because comments stay
// outside the graph a case tests.
func (w *world) loadFixture(dir string) error {
	abs := filepath.Join(w.r.root, filepath.FromSlash(dir))
	rel, err := filepath.Rel(w.r.root, abs)
	if err != nil || !filepath.IsLocal(rel) {
		return fmt.Errorf("t.fixtures: %s is outside the project", dir)
	}
	// fsimport opens the directory itself with os.OpenRoot, which follows a
	// symlink in its name; refuse one, so a fixture cannot leave the project.
	if lerr := noSymlinks(w.r.root, rel); lerr != nil {
		return fmt.Errorf("t.fixtures: %s: %w", dir, lerr)
	}
	rep, err := fsimport.Copy(w.systemContext(), fsimport.CopyOptions{
		Source: abs,
		Meta:   w.svc.Meta(),
		Target: fsimport.Target{
			Store: w.svc.Store(), State: w.svc.State(), Comments: memcomments.New(), Migrations: w.svc.MigState(),
		},
		Principal: principal.Principal{User: principal.UserFSImport, Tool: principal.ToolRelaTest},
	})
	if err != nil {
		var problems []string
		if rep != nil {
			problems = rep.Errors
		}
		if len(problems) > 0 {
			return fmt.Errorf("t.fixtures %s: %w: %s (fixtures must match the current schema)",
				dir, err, strings.Join(problems, "; "))
		}
		return fmt.Errorf("t.fixtures %s: %w", dir, err)
	}
	if len(rep.Skipped) > 0 {
		var skipped []string
		for _, sk := range rep.Skipped {
			skipped = append(skipped, sk.Path+" ("+sk.Reason+")")
		}
		return fmt.Errorf("t.fixtures %s: files not loaded: %s", dir, strings.Join(skipped, "; "))
	}
	if rep.Entities == 0 && rep.Relations == 0 {
		return fmt.Errorf("t.fixtures %s: no entities or relations found; "+
			"entity files go in entities/<plural>/ and relation files in relations/", dir)
	}
	return nil
}

// as returns ctx carrying user as a `rela test` principal, bound to the
// world's ACL. It builds the world first.
//
// A principal declared with t.principal acts as its assignee: with
// principal_property that is its user entity's id, the identity the web
// edge resolves the name to. The binder below takes the name as the graph
// identity as it stands, like the CLI does, so it is resolved here.
func (w *world) as(user string) (context.Context, error) {
	if err := w.ensure(); err != nil {
		return nil, err
	}
	for _, tp := range w.principals {
		if tp.name == user {
			user = tp.assignee
			break
		}
	}
	ctx := principal.With(w.ctx, principal.VerifiedFrom(user, principal.ToolRelaTest, principal.Claims{}))
	return w.bind(ctx)
}

// anyExists reports whether the store holds an entity, at any face, with
// one of ids. It reads raw, past the ACL.
func (w *world) anyExists(ctx context.Context, ids []string) bool {
	q := store.EntityQuery{IDs: ids, Faces: store.AllFaces()}
	for e, err := range w.svc.Store().ListEntities(
		principal.With(ctx, principal.Principal{User: principal.UserFSImport, Tool: principal.ToolRelaTest}), q) {
		if err == nil && e != nil {
			return true
		}
	}
	return false
}

// fail records the case's first failed assertion.
func (w *world) fail(where, msg string) {
	if w.failure == nil {
		w.failure = &failure{where: where, msg: msg}
	}
}

// attachGenerated attaches a generated file to each of e's file properties
// named in props, through the attachment service, and returns e as the
// actor reads it afterwards. The attachment service's own re-check is
// skipped (the create was authorized as the actor), but the property write
// still goes through the entity manager as the actor, so it needs update on
// the type, as an upload after a create does in production.
func (w *world) attachGenerated(ctx context.Context, e *entity.Entity, props []string) (*entity.Entity, error) {
	owner, err := entitymanager.AttachmentsOf(w.svc.EntityManager())
	if err != nil {
		return nil, err
	}
	att, err := attachment.New(attachment.Deps{
		Store: w.svc.Store(), Meta: w.svc.Meta(), EntityManager: owner, Locker: owner,
		Authorizer: attachment.AllowAllWrites{},
	})
	if err != nil {
		return nil, err
	}
	def := w.svc.Meta().Entities[e.Type]
	for _, prop := range props {
		w.fake.files++
		name := fmt.Sprintf("generated-%d.txt", w.fake.files)
		if _, werr := att.WriteAttachment(ctx, e, def.Properties[prop], prop, name,
			strings.NewReader(generatedFile)); werr != nil {
			return nil, fmt.Errorf("attach a generated file to %s.%s: %w", e.Type, prop, werr)
		}
	}
	addr := e.ID
	if e.Face != "" {
		addr += "@" + string(e.Face)
	}
	read, err := w.svc.GatedReads().Reader.Resolve(ctx, addr)
	if errors.Is(err, store.ErrNotFound) {
		// The actor may create the type but not read it: give back what the
		// create returned, as the write path does.
		return e, nil
	}
	return read, err
}
