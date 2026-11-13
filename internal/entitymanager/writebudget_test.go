package entitymanager_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/statemachine"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/store/storetest"
)

// writeBudgetSchema has one type per id style the create path mints, and a
// `unique:` property, so a budget test covers id generation and the unique
// check together.
const writeBudgetSchema = `version: "1.0"
entities:
  note:
    label: Note
    plural: notes
    id_prefix: "NOTE-"
    properties:
      title:
        type: string
  requirement:
    label: Requirement
    plural: requirements
    id_prefix: "REQ-"
    id_type: sequential
    properties:
      title:
        type: string
  person:
    label: Person
    plural: persons
    id_prefix: "P-"
    id_type: sequential
    properties:
      email:
        type: string
        unique: true
`

// newWriteBudgetManager returns a manager over a counting memstore padded
// with n rows of every type in [writeBudgetSchema]: what a whole-type or
// whole-store scan would return.
func newWriteBudgetManager(t *testing.T, n int) (*entitymanager.Manager, *storetest.Counting) {
	t.Helper()
	meta, err := metamodel.Parse([]byte(writeBudgetSchema))
	if err != nil {
		t.Fatalf("metamodel.Parse: %v", err)
	}
	mem := memstore.New()
	ctx := context.Background()
	for i := range n {
		seed := []*entity.Entity{
			entity.New(fmt.Sprintf("NOTE-PAD%04d", i), "note"),
			entity.New(fmt.Sprintf("REQ-%03d", i+1), "requirement"),
			entity.New(fmt.Sprintf("P-%03d", i+1), "person"),
		}
		seed[2].SetString("email", fmt.Sprintf("pad%d@example.com", i))
		for _, e := range seed {
			if cerr := mem.CreateEntity(ctx, e); cerr != nil {
				t.Fatalf("seed %s: %v", e.ID, cerr)
			}
		}
	}
	counting := storetest.NewCounting(mem)
	mgr, err := entitymanager.New(entitymanager.Deps{
		Store:       counting,
		Meta:        meta,
		Templater:   nopTemplater{},
		Audit:       audit.Nop{},
		ACL:         acl.NopACL{},
		Transitions: statemachine.EmptySet(),
		FieldGate:   entitymanager.AllowAllFieldGate{},
		AnchorGuard: entitymanager.NopAnchorGuard{},
	})
	if err != nil {
		t.Fatalf("entitymanager.New: %v", err)
	}
	return mgr, counting
}

// TestCreate_CostIsScaleFree pins TKT-FED6FMM on the write path: minting an
// id and checking `unique:` read the same rows over a store of 100 and of
// 1,000 rows per type. Before it, a create read every entity of the store
// to mint an id on every backend but fsstore, and every entity of the type
// to check a unique value.
func TestCreate_CostIsScaleFree(t *testing.T) {
	cases := []struct {
		name  string
		build func() *entity.Entity
		check func(t *testing.T, n int, got *entity.Entity)
	}{
		{
			name:  "short id",
			build: func() *entity.Entity { return entity.New("", "note") },
			check: func(t *testing.T, _ int, got *entity.Entity) {
				t.Helper()
				if !strings.HasPrefix(got.ID, "NOTE-") || strings.HasPrefix(got.ID, "NOTE-PAD") {
					t.Fatalf("minted %q", got.ID)
				}
			},
		},
		{
			name:  "sequential id",
			build: func() *entity.Entity { return entity.New("", "requirement") },
			check: func(t *testing.T, n int, got *entity.Entity) {
				t.Helper()
				if want := fmt.Sprintf("REQ-%03d", n+1); got.ID != want {
					t.Fatalf("minted %q, want %q", got.ID, want)
				}
			},
		},
		{
			name: "unique value",
			build: func() *entity.Entity {
				e := entity.New("", "person")
				e.SetString("email", "new@example.com")
				return e
			},
			check: func(*testing.T, int, *entity.Entity) {},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			storetest.AssertScaleFree(t, func(n int) (storetest.Cost, string) {
				mgr, counting := newWriteBudgetManager(t, n)
				counting.Reset()
				res, err := mgr.CreateEntity(context.Background(), tc.build(), entity.CreateOptions{})
				if err != nil {
					t.Fatalf("create: %v", err)
				}
				tc.check(t, n, res.Entity)
				return counting.Cost(), counting.String()
			})
		})
	}
}

// TestCreate_UniqueLookupStillRefusesDuplicate pins that the equality
// lookup finds a collision among many rows, at the writer's face only.
func TestCreate_UniqueLookupStillRefusesDuplicate(t *testing.T) {
	mgr, _ := newWriteBudgetManager(t, 50)
	e := entity.New("", "person")
	e.SetString("email", "pad17@example.com")
	_, err := mgr.CreateEntity(context.Background(), e, entity.CreateOptions{})
	if err == nil || !strings.Contains(err.Error(), "must be unique") {
		t.Fatalf("duplicate email: err = %v, want a unique violation", err)
	}
}
