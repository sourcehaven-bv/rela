package entitymanager_test

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// familyRenamePolicy holds the roles the rename tests act as. A rename of a
// faced type is decided once, from the family-level `rename:` grant, so
// update on one face, on every face, or on the type does not rename.
// renamer holds the grant; blindrenamer holds it but reads only the
// published face; pubeditor reads and updates only the published face.
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
  renamer:
    read: ["*"]
    rename: ["policy"]
  blindrenamer:
    read: ["policy@published"]
    rename: ["policy"]
  pubeditor:
    read: ["policy@published"]
    update: ["policy@published"]
assignments:
  typewide: typewide
  drafter: drafter
  publisher: publisher
  editor: editor
  renamer: renamer
  blindrenamer: blindrenamer
  pubeditor: pubeditor
`

// facesAt returns the faces of id that are stored.
func (f familyDeleteFixture) facesAt(t *testing.T, id string) []entity.Face {
	t.Helper()
	var faces []entity.Face
	for _, face := range bothFaces {
		_, err := f.st.GetEntity(context.Background(), entity.Ref{ID: id, Face: face})
		switch {
		case err == nil:
			faces = append(faces, face)
		case !errors.Is(err, store.ErrNotFound):
			t.Fatalf("GetEntity %s@%s: %v", id, face, err)
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

// A rename re-keys the whole family, hidden faces included, so it is
// authorized once from the family-level `rename:` grant (BUG-Y1RGTU, then
// the family rule). A grant on each face does not rename: deciding per
// stored face would consult faces the caller cannot read. The denial names
// the type and no face.
func TestFamilyRename_DeniedWithoutTheFamilyGrant(t *testing.T) {
	for _, b := range concBackends {
		for _, user := range []string{"typewide", "drafter", "publisher", "editor"} {
			t.Run(b.name+"/"+user, func(t *testing.T) {
				f := newFacedPolicyFixture(t, b, nil, familyRenamePolicy, bothFaces...)

				_, err := f.mgr.RenameEntity(asUser(user), "POL-1", "POL-2", entity.RenameOptions{})

				var forbidden *acl.ForbiddenError
				if !errors.As(err, &forbidden) {
					t.Fatalf("RenameEntity as %s = %v, want *acl.ForbiddenError", user, err)
				}
				for _, face := range bothFaces {
					if strings.Contains(err.Error(), string(face)) {
						t.Errorf("denial %q names the face %q", err, face)
					}
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
			})
		}
	}
}

// renameOutcome is what a caller observes of a rename: the error text, or
// the result.
func renameOutcome(res *entity.RenameResult, err error) string {
	if err != nil {
		return "error: " + err.Error()
	}
	return fmt.Sprintf("ok: %+v", *res)
}

// A caller who cannot read the draft face observes the same rename whether
// or not a draft exists: the same refusal without the family grant, the
// same success with it. With the grant the hidden draft moves too, because
// a rename moves the whole family.
func TestFamilyRename_HiddenFaceIsNoOracle(t *testing.T) {
	for _, b := range concBackends {
		for _, user := range []string{"pubeditor", "blindrenamer"} {
			t.Run(b.name+"/"+user, func(t *testing.T) {
				outcomes := map[string]string{}
				for _, seeded := range [][]entity.Face{{"published"}, bothFaces} {
					f := newFacedPolicyFixture(t, b, nil, familyRenamePolicy, seeded...)
					res, err := f.mgr.RenameEntity(asUser(user), "POL-1", "POL-2", entity.RenameOptions{})
					outcomes[fmt.Sprint(seeded)] = renameOutcome(res, err)
					if err == nil {
						if got := f.facesAt(t, "POL-2"); !slices.Equal(got, seeded) {
							t.Errorf("POL-2 faces after the rename = %v, want %v", got, seeded)
						}
					}
					if strings.Contains(outcomes[fmt.Sprint(seeded)], "draft") {
						t.Errorf("outcome %q names the hidden face", outcomes[fmt.Sprint(seeded)])
					}
				}
				if a, b := outcomes["[published]"], outcomes["[draft published]"]; a != b {
					t.Errorf("rename without a hidden draft = %q, with one = %q; want the same", a, b)
				}
			})
		}
	}
}

// A family grant holder who can read no face of the entity gets the same
// not-found as for an entity that does not exist, and the attempt is not
// authorized, so no denied-write record says it was.
func TestFamilyRename_NoReadableFaceIsNotFound(t *testing.T) {
	for _, b := range concBackends {
		t.Run(b.name, func(t *testing.T) {
			f := newFacedPolicyFixture(t, b, nil, familyRenamePolicy, "draft")

			_, err := f.mgr.RenameEntity(asUser("blindrenamer"), "POL-1", "POL-2", entity.RenameOptions{})
			if !errors.Is(err, entitymanager.ErrEntityNotFound) {
				t.Fatalf("rename of an unreadable entity = %v, want ErrEntityNotFound", err)
			}
			_, missing := f.mgr.RenameEntity(asUser("blindrenamer"), "POL-9", "POL-2", entity.RenameOptions{})
			if err.Error() != strings.ReplaceAll(missing.Error(), "POL-9", "POL-1") {
				t.Errorf("unreadable = %q, missing = %q; want the same message", err, missing)
			}
			if got := f.facesAt(t, "POL-1"); !slices.Equal(got, []entity.Face{"draft"}) {
				t.Errorf("POL-1 faces = %v, want the draft untouched", got)
			}
			if n := len(f.aud.Records()); n != 0 {
				t.Errorf("rename of an unreadable entity wrote %d audit records, want 0", n)
			}
		})
	}
}

// A dry run is refused on the same terms as the rename it previews, and with
// the family grant it plans the faced rename without writing.
func TestFamilyRename_DryRun(t *testing.T) {
	for _, b := range concBackends {
		t.Run(b.name, func(t *testing.T) {
			f := newFacedPolicyFixture(t, b, nil, familyRenamePolicy, bothFaces...)
			dry := entity.RenameOptions{DryRun: true}

			var forbidden *acl.ForbiddenError
			if _, err := f.mgr.RenameEntity(asUser("editor"), "POL-1", "POL-2", dry); !errors.As(err, &forbidden) {
				t.Errorf("dry-run rename as editor = %v, want *acl.ForbiddenError", err)
			}
			res, err := f.mgr.RenameEntity(asUser("renamer"), "POL-1", "POL-2", dry)
			if err != nil {
				t.Fatalf("dry-run rename as renamer: %v", err)
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
				_, err = f.mgr.RenameEntity(asUser("renamer"), "POL-1", target, dry)
				if !errors.Is(err, entitymanager.ErrEntityAlreadyExists) {
					t.Errorf("dry-run rename onto %s = %v, want ErrEntityAlreadyExists", target, err)
				}
			}
			if _, err = f.mgr.RenameEntity(asUser("renamer"), "POL-1", "Pol-1", dry); err != nil {
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

			if _, err := f.mgr.RenameEntity(asUser("renamer"), "POL-1", "Pol-1", entity.RenameOptions{}); err != nil {
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

// A family grant holder's rename moves both faces and leaves one rename
// version and one audit record per face.
func TestFamilyRename_EveryFaceCapturedAndAudited(t *testing.T) {
	for _, b := range concBackends {
		t.Run(b.name, func(t *testing.T) {
			f := newFacedPolicyFixture(t, b, nil, familyRenamePolicy, bothFaces...)

			if _, err := f.mgr.RenameEntity(asUser("renamer"), "POL-1", "POL-2", entity.RenameOptions{}); err != nil {
				t.Fatalf("RenameEntity as renamer: %v", err)
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

// RenameFamily on the Tx view stands in for the store's rename, so
// injectBeforeStoreWrite also adds the face just before a family rename.
func (tx *faceInjectingTx) RenameFamily(ctx context.Context, oldID, newID string) (*store.RenameResult, error) {
	if tx.parent.at == injectBeforeStoreWrite {
		if err := tx.CreateEntity(ctx, policyFace("published")); err != nil {
			tx.parent.t.Errorf("inject published face: %v", err)
		}
	}
	return tx.Store.RenameFamily(ctx, oldID, newID)
}

// The family decision does not depend on which faces exist, so a face that
// appears during the rename changes nothing: without the grant the rename is
// refused, and with it the family moves whole, never split across ids. A
// backend that rolls back leaves nothing to record; one that cannot records
// exactly the faces it moved.
func TestFamilyRename_FaceAddedDuringRename(t *testing.T) {
	for _, b := range concBackends {
		for _, tc := range []struct {
			name string
			at   injectAt
		}{
			{"before-tx", injectBeforeTx},
			{"before-store-rename", injectBeforeStoreWrite},
		} {
			for _, user := range []string{"drafter", "renamer"} {
				t.Run(b.name+"/"+tc.name+"/"+user, func(t *testing.T) {
					wrap := func(st store.Store) store.Store {
						return &faceInjectingStore{Store: st, at: tc.at, t: t}
					}
					f := newFacedPolicyFixture(t, b, wrap, familyRenamePolicy, "draft")

					_, err := f.mgr.RenameEntity(asUser(user), "POL-1", "POL-2", entity.RenameOptions{})

					var forbidden *acl.ForbiddenError
					if denied := errors.As(err, &forbidden); denied != (user == "drafter") {
						t.Fatalf("RenameEntity as %s = %v", user, err)
					}
					if err != nil && !errors.As(err, &forbidden) {
						t.Fatalf("RenameEntity as %s: %v", user, err)
					}
					oldFaces, newFaces := f.facesAt(t, "POL-1"), f.facesAt(t, "POL-2")
					if len(newFaces) != 0 && (len(oldFaces) != 0 || !slices.Equal(newFaces, bothFaces)) {
						t.Errorf("faces split by the rename: POL-1 %v, POL-2 %v", oldFaces, newFaces)
					}
					if user == "drafter" && len(newFaces) != 0 {
						t.Errorf("a refused rename moved faces %v", newFaces)
					}
					want := map[entity.Face]int{}
					for _, face := range newFaces {
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
}
