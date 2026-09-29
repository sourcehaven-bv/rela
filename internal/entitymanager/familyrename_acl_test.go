package entitymanager_test

import (
	"context"
	"errors"
	"maps"
	"slices"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// familyRenamePolicy grants update, the verb a rename is checked against,
// per face. typewide holds the bare type grant, which covers the zero face
// only; editor holds every face.
const familyRenamePolicy = `
roles:
  typewide:
    read: ["*"]
    update: ["policy"]
  drafter:
    read: ["*"]
    update: ["policy@draft"]
  publisher:
    read: ["*"]
    update: ["policy@published"]
  editor:
    read: ["*"]
    update: ["policy@draft", "policy@published"]
assignments:
  typewide: typewide
  drafter: drafter
  publisher: publisher
  editor: editor
`

// facesAt returns the faces of id that are stored.
func (f familyDeleteFixture) facesAt(t *testing.T, id string) []entity.Face {
	t.Helper()
	var faces []entity.Face
	for _, face := range bothFaces {
		_, err := f.st.GetEntityState(context.Background(), id, face)
		switch {
		case err == nil:
			faces = append(faces, face)
		case !errors.Is(err, store.ErrNotFound):
			t.Fatalf("GetEntityState %s@%s: %v", id, face, err)
		}
	}
	return faces
}

// renameVersionFaces and renameRecordFaces return, per face, how many rename
// versions and rename-entity audit records the fixture captured.
func (f familyDeleteFixture) renameVersionFaces() map[entity.Face]int {
	got := map[entity.Face]int{}
	for _, v := range f.rec.records {
		if v.Op == store.VersionOpRename && v.EntityID == "POL-2" && v.PrevID == "POL-1" {
			got[v.Face]++
		}
	}
	return got
}

func (f familyDeleteFixture) renameRecordFaces() map[entity.Face]int {
	got := map[entity.Face]int{}
	for _, r := range f.aud.Records() {
		if r.Op != audit.OpRenameEntity {
			continue
		}
		for _, face := range bothFaces {
			if r.Summary == "renamed face "+string(face) {
				got[face]++
			}
		}
	}
	return got
}

// A rename re-keys the whole family, so it must be authorized on every face
// it moves (BUG-Y1RGTU). Before the fix the check ran against the zero face,
// which a faced type does not store: a bare `update: [policy]` grant renamed
// the published face along with the rest.
func TestFamilyRename_DeniedUnlessEveryFaceIsRenamable(t *testing.T) {
	for _, b := range concBackends {
		for _, tc := range []struct {
			user   string
			denied entity.Face
		}{
			{"typewide", "draft"},
			{"drafter", "published"},
			{"publisher", "draft"},
		} {
			t.Run(b.name+"/"+tc.user, func(t *testing.T) {
				f := newFacedPolicyFixture(t, b, nil, familyRenamePolicy, bothFaces...)

				_, err := f.mgr.RenameEntity(asUser(tc.user), "POL-1", "POL-2", entity.RenameOptions{})

				var forbidden *acl.ForbiddenError
				if !errors.As(err, &forbidden) {
					t.Fatalf("RenameEntity as %s = %v, want *acl.ForbiddenError", tc.user, err)
				}
				if got := f.facesAt(t, "POL-1"); !slices.Equal(got, bothFaces) {
					t.Errorf("POL-1 faces after a denied rename = %v, want both", got)
				}
				if got := f.facesAt(t, "POL-2"); len(got) != 0 {
					t.Errorf("POL-2 faces after a denied rename = %v, want none", got)
				}
				if len(f.rec.records) != 0 {
					t.Errorf("a denied rename captured %d versions, want 0", len(f.rec.records))
				}
				var denied int
				for _, r := range f.aud.Records() {
					switch r.Op {
					case audit.OpRenameEntity:
						t.Errorf("a denied rename wrote a rename-entity audit record")
					case audit.OpDeniedWrite:
						denied++
					}
				}
				if denied != 1 {
					t.Errorf("denied-write records = %d, want 1", denied)
				}
				if !slices.Equal(f.gate.denied, []entity.Face{tc.denied}) {
					t.Errorf("denied faces = %v, want [%s]", f.gate.denied, tc.denied)
				}
			})
		}
	}
}

// A dry run is refused on the same terms as the rename it previews, and with
// every face granted it plans the faced rename without writing.
func TestFamilyRename_DryRun(t *testing.T) {
	for _, b := range concBackends {
		t.Run(b.name, func(t *testing.T) {
			f := newFacedPolicyFixture(t, b, nil, familyRenamePolicy, bothFaces...)
			dry := entity.RenameOptions{DryRun: true}

			var forbidden *acl.ForbiddenError
			if _, err := f.mgr.RenameEntity(asUser("drafter"), "POL-1", "POL-2", dry); !errors.As(err, &forbidden) {
				t.Errorf("dry-run rename as drafter = %v, want *acl.ForbiddenError", err)
			}
			res, err := f.mgr.RenameEntity(asUser("editor"), "POL-1", "POL-2", dry)
			if err != nil {
				t.Fatalf("dry-run rename as editor: %v", err)
			}
			if res.OldID != "POL-1" || res.NewID != "POL-2" {
				t.Errorf("dry-run result = %+v, want POL-1 -> POL-2", res)
			}
			if got := f.facesAt(t, "POL-1"); !slices.Equal(got, bothFaces) {
				t.Errorf("POL-1 faces after a dry run = %v, want both", got)
			}
			taken := policyFace("published")
			taken.ID = "POL-3"
			if err = f.st.CreateEntity(context.Background(), taken); err != nil {
				t.Fatalf("seed POL-3@published: %v", err)
			}
			// The store folds case when it checks for a conflict, and the dry
			// run must agree: another entity's id in other case conflicts, the
			// entity's own id in other case does not.
			for _, target := range []string{"POL-3", "pol-3"} {
				_, err = f.mgr.RenameEntity(asUser("editor"), "POL-1", target, dry)
				if !errors.Is(err, entitymanager.ErrEntityAlreadyExists) {
					t.Errorf("dry-run rename onto %s = %v, want ErrEntityAlreadyExists", target, err)
				}
			}
			if _, err = f.mgr.RenameEntity(asUser("editor"), "POL-1", "Pol-1", dry); err != nil {
				t.Errorf("dry-run case-only rename: %v", err)
			}
		})
	}
}

// A case-only rename moves every face like any other rename.
func TestFamilyRename_CaseOnly(t *testing.T) {
	for _, b := range concBackends {
		t.Run(b.name, func(t *testing.T) {
			f := newFacedPolicyFixture(t, b, nil, familyRenamePolicy, bothFaces...)

			if _, err := f.mgr.RenameEntity(asUser("editor"), "POL-1", "Pol-1", entity.RenameOptions{}); err != nil {
				t.Fatalf("case-only rename: %v", err)
			}
			if got := f.facesAt(t, "Pol-1"); !slices.Equal(got, bothFaces) {
				t.Errorf("Pol-1 faces after the rename = %v, want both", got)
			}
		})
	}
}

// A rename of a missing entity reports it missing and authorizes nothing, so
// it writes no denied-write record.
func TestFamilyRename_NotFound(t *testing.T) {
	for _, b := range concBackends {
		t.Run(b.name, func(t *testing.T) {
			f := newFacedPolicyFixture(t, b, nil, familyRenamePolicy)

			_, err := f.mgr.RenameEntity(asUser("drafter"), "POL-1", "POL-2", entity.RenameOptions{})
			if !errors.Is(err, entitymanager.ErrEntityNotFound) {
				t.Fatalf("rename of a missing entity = %v, want ErrEntityNotFound", err)
			}
			if n := len(f.aud.Records()); n != 0 {
				t.Errorf("rename of a missing entity wrote %d audit records, want 0", n)
			}
		})
	}
}

// With update on every face, the rename moves both faces and leaves one
// rename version and one audit record per face.
func TestFamilyRename_EveryFaceCapturedAndAudited(t *testing.T) {
	for _, b := range concBackends {
		t.Run(b.name, func(t *testing.T) {
			f := newFacedPolicyFixture(t, b, nil, familyRenamePolicy, bothFaces...)

			if _, err := f.mgr.RenameEntity(asUser("editor"), "POL-1", "POL-2", entity.RenameOptions{}); err != nil {
				t.Fatalf("RenameEntity as editor: %v", err)
			}
			if got := f.facesAt(t, "POL-1"); len(got) != 0 {
				t.Errorf("POL-1 faces after the rename = %v, want none", got)
			}
			if got := f.facesAt(t, "POL-2"); !slices.Equal(got, bothFaces) {
				t.Errorf("POL-2 faces after the rename = %v, want both", got)
			}
			want := map[entity.Face]int{"draft": 1, "published": 1}
			if got := f.renameVersionFaces(); !maps.Equal(got, want) {
				t.Errorf("rename versions per face = %v, want %v", got, want)
			}
			if got := f.renameRecordFaces(); !maps.Equal(got, want) {
				t.Errorf("rename audit records per face = %v, want %v", got, want)
			}
		})
	}
}

// RenameEntity on the Tx view stands in for the store's rename, so
// injectBeforeStoreWrite also adds the face just before a family rename.
func (tx *faceInjectingTx) RenameEntity(ctx context.Context, oldID, newID string) (*store.RenameResult, error) {
	if tx.parent.at == injectBeforeStoreWrite {
		if err := tx.CreateEntity(ctx, policyFace("published")); err != nil {
			tx.parent.t.Errorf("inject published face: %v", err)
		}
	}
	return tx.Store.RenameEntity(ctx, oldID, newID)
}

// A face that appears after the first authorization is still authorized
// before the rename may stand. A backend that rolls back leaves nothing to
// record; one that cannot records exactly the faces it moved.
func TestFamilyRename_FaceAddedDuringRenameIsAuthorized(t *testing.T) {
	for _, b := range concBackends {
		for _, tc := range []struct {
			name string
			at   injectAt
		}{
			{"before-tx", injectBeforeTx},
			{"before-store-rename", injectBeforeStoreWrite},
		} {
			t.Run(b.name+"/"+tc.name, func(t *testing.T) {
				wrap := func(st store.Store) store.Store {
					return &faceInjectingStore{Store: st, at: tc.at, t: t}
				}
				f := newFacedPolicyFixture(t, b, wrap, familyRenamePolicy, "draft")

				_, err := f.mgr.RenameEntity(asUser("drafter"), "POL-1", "POL-2", entity.RenameOptions{})

				var forbidden *acl.ForbiddenError
				if !errors.As(err, &forbidden) {
					t.Fatalf("RenameEntity as drafter = %v, want *acl.ForbiddenError", err)
				}
				if !slices.Equal(f.gate.denied, []entity.Face{"published"}) {
					t.Errorf("denied faces = %v, want [published]", f.gate.denied)
				}
				if tc.at == injectBeforeTx {
					if got := f.facesAt(t, "POL-1"); !slices.Equal(got, bothFaces) {
						t.Errorf("POL-1 faces = %v, want both: the in-Tx check must refuse before any write", got)
					}
				}
				// The family moves whole or not at all, never split across ids.
				// A rollback also undoes the face injected inside the Tx.
				oldFaces, newFaces := f.facesAt(t, "POL-1"), f.facesAt(t, "POL-2")
				if len(newFaces) != 0 && (len(oldFaces) != 0 || !slices.Equal(newFaces, bothFaces)) {
					t.Errorf("faces split by the rename: POL-1 %v, POL-2 %v", oldFaces, newFaces)
				}
				// The log must match the store: a rename that stood (fs and
				// memstore cannot roll back) is recorded per face it moved; a
				// rename that did not is not recorded at all.
				want := map[entity.Face]int{}
				for _, face := range f.facesAt(t, "POL-2") {
					want[face] = 1
				}
				if got := f.renameVersionFaces(); !maps.Equal(got, want) {
					t.Errorf("rename versions per face = %v, want %v", got, want)
				}
				if got := f.renameRecordFaces(); !maps.Equal(got, want) {
					t.Errorf("rename audit records per face = %v, want %v", got, want)
				}
			})
		}
	}
}
