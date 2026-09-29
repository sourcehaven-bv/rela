package entitymanager_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/statemachine"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// familyDeleteMetaYAML declares a type whose every row sits at a named face,
// the realistic shape: a faced type stores no zero-face row (BUG-HC6I2T).
const familyDeleteMetaYAML = `version: "1.0"
entities:
  policy:
    label: Policy
    plural: policies
    id_prefix: "POL-"
    faces:
      draft: {label: Draft}
      published: {label: Published}
    properties:
      title: {type: string}
`

// familyDeletePolicy grants delete per face: drafter on the draft only,
// publisher on the published face only, admin on both.
const familyDeletePolicy = `
roles:
  drafter:
    read: ["*"]
    delete: ["policy@draft"]
  publisher:
    read: ["*"]
    delete: ["policy@published"]
  admin:
    read: ["*"]
    delete: ["policy@draft", "policy@published"]
assignments:
  drafter: drafter
  publisher: publisher
  admin: admin
`

var bothFaces = []entity.Face{"draft", "published"}

type familyDeleteFixture struct {
	st   store.Store
	mgr  *entitymanager.Manager
	aud  *audit.Memory
	rec  *fakeRecorder
	gate *deniedFaceACL
}

// deniedFaceACL delegates to the real policy and records the face of every
// entity subject it denies, so a test can tell which face was refused.
type deniedFaceACL struct {
	inner  acl.ACL
	denied []entity.Face
}

func (a *deniedFaceACL) AuthorizeWrite(ctx context.Context, req acl.WriteRequest) acl.Decision {
	d := a.inner.AuthorizeWrite(ctx, req)
	if es, ok := req.Subject.(acl.EntitySubject); ok && !d.Allow {
		a.denied = append(a.denied, es.Face())
	}
	return d
}

// newFamilyDeleteFixture seeds POL-1 at the given faces on b's store. wrap,
// when non-nil, decorates the store the manager writes through; the ACL and
// the assertions read the undecorated store.
func newFamilyDeleteFixture(
	t *testing.T, b concBackend, wrap func(store.Store) store.Store, faces ...entity.Face,
) familyDeleteFixture {
	t.Helper()
	return newFacedPolicyFixture(t, b, wrap, familyDeletePolicy, faces...)
}

// newFacedPolicyFixture is newFamilyDeleteFixture with the ACL policy given.
func newFacedPolicyFixture(
	t *testing.T, b concBackend, wrap func(store.Store) store.Store, policy string, faces ...entity.Face,
) familyDeleteFixture {
	t.Helper()
	st := b.open(t)
	meta, err := metamodel.Parse([]byte(familyDeleteMetaYAML))
	if err != nil {
		t.Fatalf("metamodel.Parse: %v", err)
	}
	p, err := acl.LoadPolicyBytes([]byte(policy))
	if err != nil {
		t.Fatalf("load policy: %v", err)
	}
	d, err := acl.NewDeclarative(p, acl.NewStoreGraph(st), st)
	if err != nil {
		t.Fatalf("NewDeclarative: %v", err)
	}
	managed := st
	if wrap != nil {
		managed = wrap(st)
	}
	aud := audit.NewMemory()
	rec := &fakeRecorder{}
	gate := &deniedFaceACL{inner: d}
	mgr, err := entitymanager.New(entitymanager.Deps{
		Store: managed, Meta: meta, Templater: nopTemplater{}, Audit: aud,
		ACL: gate, Transitions: statemachine.EmptySet(),
		FieldGate:       entitymanager.AllowAllFieldGate{},
		CopyReadGate:    entitymanager.AllowAllCopyReadGate{},
		CopyVisibility:  allowAllCopyVisibility(t, st),
		VersionRecorder: rec,
	})
	if err != nil {
		t.Fatalf("entitymanager.New: %v", err)
	}
	ctx := context.Background()
	for _, face := range faces {
		if cErr := st.CreateEntity(ctx, policyFace(face)); cErr != nil {
			t.Fatalf("seed POL-1@%s: %v", face, cErr)
		}
	}
	return familyDeleteFixture{st: st, mgr: mgr, aud: aud, rec: rec, gate: gate}
}

func policyFace(face entity.Face) *entity.Entity {
	return &entity.Entity{
		ID: "POL-1", Type: "policy", Face: face,
		Properties: map[string]any{"title": "Access policy " + string(face)},
	}
}

func (f familyDeleteFixture) facesLeft(t *testing.T) []entity.Face {
	t.Helper()
	var faces []entity.Face
	for _, face := range bothFaces {
		_, err := f.st.GetEntity(context.Background(), entity.Ref{ID: "POL-1", Face: face})
		switch {
		case err == nil:
			faces = append(faces, face)
		case !errors.Is(err, store.ErrNotFound):
			t.Fatalf("GetEntityState POL-1@%s: %v", face, err)
		}
	}
	return faces
}

// assertLogMatchesStore checks that the version and audit logs agree with
// the store after a failed delete that started from a stored draft face. If
// the draft is still stored, the delete did not happen (refused, or rolled
// back) and no face may have a delete version or record. If the draft is
// gone, the backend could not roll back, and every face it removed must have
// exactly one of each.
func (f familyDeleteFixture) assertLogMatchesStore(t *testing.T) {
	t.Helper()
	left := f.facesLeft(t)
	deleted := !slices.Contains(left, "draft")
	for _, face := range bothFaces {
		versions := 0
		for _, v := range f.rec.records {
			if v.Op == store.VersionOpDelete && v.Face == face {
				versions++
			}
		}
		records := 0
		for _, r := range f.aud.Records() {
			if r.Op == audit.OpDeleteEntity && strings.Contains(r.Summary, "face "+string(face)) {
				records++
			}
		}
		want := 0
		if deleted && !slices.Contains(left, face) {
			want = 1
		}
		if versions != want || records != want {
			t.Errorf("face %s (faces left %v): %d delete versions and %d delete records, want %d of each",
				face, left, versions, records, want)
		}
	}
}

// A bare-id delete removes the whole family, so it must be authorized on
// every face it removes (BUG-1YN750). Holding delete on one face is not
// enough, whichever face that is: before the fix the check ran against the
// single face anyFaceOf returned, which let a draft-only principal delete
// the published face.
func TestFamilyDelete_DeniedUnlessEveryFaceIsDeletable(t *testing.T) {
	for _, b := range concBackends {
		for _, user := range []string{"drafter", "publisher"} {
			t.Run(b.name+"/"+user, func(t *testing.T) {
				f := newFamilyDeleteFixture(t, b, nil, bothFaces...)

				_, err := f.mgr.DeleteEntity(asUser(user), "POL-1", false)

				var forbidden *acl.ForbiddenError
				if !errors.As(err, &forbidden) {
					t.Fatalf("DeleteEntity as %s = %v, want *acl.ForbiddenError", user, err)
				}
				if got := f.facesLeft(t); !slices.Equal(got, bothFaces) {
					t.Errorf("faces left after a denied delete = %v, want both", got)
				}
				if len(f.rec.records) != 0 {
					t.Errorf("a denied delete captured %d versions, want 0", len(f.rec.records))
				}
				var denied int
				for _, r := range f.aud.Records() {
					switch r.Op {
					case audit.OpDeleteEntity, audit.OpDeleteRelation:
						t.Errorf("a denied delete wrote a %s audit record", r.Op)
					case audit.OpDeniedWrite:
						denied++
					}
				}
				if denied != 1 {
					t.Errorf("denied-write records = %d, want 1", denied)
				}
				// The refusal is on the face the role lacks, not the one it holds.
				want := entity.Face("published")
				if user == "publisher" {
					want = "draft"
				}
				if !slices.Equal(f.gate.denied, []entity.Face{want}) {
					t.Errorf("denied faces = %v, want [%s]", f.gate.denied, want)
				}
			})
		}
	}
}

// With delete on every face, the family delete removes both faces and leaves
// one version capture and one audit record per removed face.
func TestFamilyDelete_EveryFaceCapturedAndAudited(t *testing.T) {
	for _, b := range concBackends {
		t.Run(b.name, func(t *testing.T) {
			f := newFamilyDeleteFixture(t, b, nil, bothFaces...)

			res, err := f.mgr.DeleteEntity(asUser("admin"), "POL-1", false)
			if err != nil {
				t.Fatalf("DeleteEntity as admin: %v", err)
			}
			if got := f.facesLeft(t); len(got) != 0 {
				t.Errorf("faces left after the family delete = %v, want none", got)
			}
			if len(res.DeletedEntities) != 2 {
				t.Errorf("DeletedEntities = %d rows, want 2", len(res.DeletedEntities))
			}

			var versioned []entity.Face
			for _, v := range f.rec.records {
				if v.Op != store.VersionOpDelete || v.EntityID != "POL-1" {
					t.Errorf("unexpected version record %+v", v)
					continue
				}
				if want := "Access policy " + string(v.Face); v.Properties["title"] != want {
					t.Errorf("version of face %q carries title %v, want %q", v.Face, v.Properties["title"], want)
				}
				versioned = append(versioned, v.Face)
			}
			slices.Sort(versioned)
			if !slices.Equal(versioned, bothFaces) {
				t.Errorf("delete versions captured for faces %v, want [draft published]", versioned)
			}

			var audited []string
			for _, r := range f.aud.Records() {
				if r.Op == audit.OpDeleteEntity {
					audited = append(audited, r.Summary)
				}
			}
			if len(audited) != 2 {
				t.Fatalf("delete-entity audit records = %v, want one per face", audited)
			}
			for _, face := range []string{"draft", "published"} {
				if !slices.ContainsFunc(audited, func(s string) bool { return strings.Contains(s, "face "+face) }) {
					t.Errorf("no delete-entity audit record names face %s: %v", face, audited)
				}
			}
		})
	}
}

// injectAt names where faceInjectingStore adds a face during a delete.
type injectAt int

const (
	// injectBeforeTx adds the face after the manager's first family read and
	// before the transaction starts, so only the in-Tx re-read can see it.
	injectBeforeTx injectAt = iota
	// injectBeforeStoreWrite adds the face inside the transaction just before
	// the store delete or rename. It stands in for the fs watcher indexing an
	// external file edit: only the check of what the store wrote sees it.
	injectBeforeStoreWrite
)

// faceInjectingStore adds POL-1@published at one point of a family delete.
type faceInjectingStore struct {
	store.Store
	at injectAt
	t  *testing.T
}

func (s *faceInjectingStore) Tx(ctx context.Context, fn func(store.Store) error) error {
	if s.at == injectBeforeTx {
		if err := s.CreateEntity(ctx, policyFace("published")); err != nil {
			s.t.Errorf("inject published face: %v", err)
		}
	}
	return s.Store.Tx(ctx, func(tx store.Store) error {
		return fn(&faceInjectingTx{Store: tx, parent: s})
	})
}

type faceInjectingTx struct {
	store.Store
	parent *faceInjectingStore
}

func (tx *faceInjectingTx) DeleteEntity(
	ctx context.Context, id string, cascade bool,
) (*store.DeleteResult, error) {
	if tx.parent.at == injectBeforeStoreWrite {
		if err := tx.CreateEntity(ctx, policyFace("published")); err != nil {
			tx.parent.t.Errorf("inject published face: %v", err)
		}
	}
	return tx.Store.DeleteEntity(ctx, id, cascade)
}

// A face that appears after the first authorization is still authorized
// before the family delete may stand. A backend that rolls back leaves
// nothing to record; one that cannot records exactly what it removed.
func TestFamilyDelete_FaceAddedDuringDeleteIsAuthorized(t *testing.T) {
	for _, b := range concBackends {
		for _, tc := range []struct {
			name string
			at   injectAt
		}{
			{"before-tx", injectBeforeTx},
			{"before-store-delete", injectBeforeStoreWrite},
		} {
			t.Run(b.name+"/"+tc.name, func(t *testing.T) {
				wrap := func(st store.Store) store.Store {
					return &faceInjectingStore{Store: st, at: tc.at, t: t}
				}
				f := newFamilyDeleteFixture(t, b, wrap, "draft")

				_, err := f.mgr.DeleteEntity(asUser("drafter"), "POL-1", false)

				var forbidden *acl.ForbiddenError
				if !errors.As(err, &forbidden) {
					t.Fatalf("DeleteEntity as drafter = %v, want *acl.ForbiddenError", err)
				}
				if !slices.Equal(f.gate.denied, []entity.Face{"published"}) {
					t.Errorf("denied faces = %v, want [published]", f.gate.denied)
				}
				if tc.at == injectBeforeTx {
					if got := f.facesLeft(t); !slices.Equal(got, bothFaces) {
						t.Errorf("faces left = %v, want both: the in-Tx check must refuse before any write", got)
					}
				}
				f.assertLogMatchesStore(t)
			})
		}
	}
}
