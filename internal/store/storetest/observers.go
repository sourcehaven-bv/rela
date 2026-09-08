package storetest

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// ObserverFactory returns a fresh store with the given observers registered
// at construction (observers cannot be added later on any backend).
type ObserverFactory func(t *testing.T, obs ...store.EntityObserver) store.Store

// faceRecorder is the bare-id observer: the pre-worlds contract, which
// must still hold — one delete when the ENTITY goes, never a per-face delete
// it could not act on.
type faceRecorder struct {
	puts    []string
	deletes []string
	renames []string
}

func (r *faceRecorder) EntityPut(e *entity.Entity) error {
	r.puts = append(r.puts, e.ID+"|"+e.Face.String())
	return nil
}

func (r *faceRecorder) EntityDelete(id string) error {
	r.deletes = append(r.deletes, id)
	return nil
}

func (r *faceRecorder) EntityRenamed(oldID string, renamed *entity.Entity) error {
	r.renames = append(r.renames, oldID+"->"+renamed.ID+"|"+renamed.Face.String())
	return nil
}

// faceAwareRecorder implements the optional [store.FaceObserver]
// capability, so the store addresses deletes to it by (id, face).
type faceAwareRecorder struct {
	faceRecorder
	faceDeletes []string
}

func (r *faceAwareRecorder) EntityFaceDelete(id string, p entity.Face) error {
	r.faceDeletes = append(r.faceDeletes, id+"|"+p.String())
	return nil
}

// RunObserverTests pins the observer contract every backend must honor
// (store.EntityObserver + store.FaceObserver). It lives HERE, not in each
// backend's own tests, because the contract once drifted exactly that way:
// memstore and fsstore pinned per-face announcements while pgstore pinned the
// pre-worlds skip, and each backend's tests were green.
func RunObserverTests(t *testing.T, f ObserverFactory) {
	face := func(t *testing.T, v string) entity.Face {
		t.Helper()
		p, err := entity.ParseFace(v)
		require.NoError(t, err)
		return p
	}

	// Indexes key documents per FACE, so an index that never hears about a
	// draft cannot search a world that selects drafts. Every face is
	// announced: the default create, the draft create, the draft update.
	t.Run("AnnounceEveryFace", func(t *testing.T) {
		obs := &faceRecorder{}
		s := f(t, obs)
		def := entity.New("PAGE-1", "page")
		require.NoError(t, s.CreateEntity(ctx(), def))
		draft := entity.New("PAGE-1", "page")
		draft.Face = face(t, "draft")
		require.NoError(t, s.CreateEntity(ctx(), draft))
		draft.SetString("title", "edited")
		require.NoError(t, s.UpdateEntity(ctx(), draft))

		assert.Equal(t, []string{"PAGE-1|", "PAGE-1|draft", "PAGE-1|draft"}, obs.puts)
	})

	// The two observer kinds are MUTUALLY EXCLUSIVE per delete: a face-aware
	// observer learns which face went; a bare-id observer hears nothing until
	// the ENTITY is gone, because a bare-id delete cannot address a face and
	// acting on one would de-index a live entity.
	t.Run("FaceDeleteAddressesOneFace", func(t *testing.T) {
		bare := &faceRecorder{}
		aware := &faceAwareRecorder{}
		s := f(t, bare, aware)
		require.NoError(t, s.CreateEntity(ctx(), entity.New("PAGE-1", "page")))
		draft := entity.New("PAGE-1", "page")
		draft.Face = face(t, "draft")
		require.NoError(t, s.CreateEntity(ctx(), draft))

		_, err := s.DeleteEntityState(ctx(), "PAGE-1", face(t, "draft"))
		require.NoError(t, err)
		assert.Equal(t, []string{"PAGE-1|draft"}, aware.faceDeletes,
			"the face-aware observer is told WHICH face went")
		assert.Empty(t, aware.deletes,
			"a face-aware observer must not ALSO get the bare-id delete")
		assert.Empty(t, bare.deletes,
			"the entity still exists via its default face, so a bare-id observer hears nothing")

		_, err = s.DeleteEntity(ctx(), "PAGE-1", false)
		require.NoError(t, err)
		assert.Equal(t, []string{"PAGE-1"}, bare.deletes,
			"the bare-id observer hears exactly one delete, when the entity goes")
		assert.Contains(t, aware.faceDeletes, "PAGE-1|",
			"the face-aware observer is told about the default face too")
		assert.Empty(t, aware.deletes,
			"even when the WHOLE entity goes, a face-aware observer hears only per-face deletes")
	})

	// Every face is renamed, default face FIRST: an index implements
	// EntityRenamed as "drop the old family, insert this face", so the first
	// call removes and the later ones add siblings back. A rename that
	// announced only the default face would strand every sibling under an id
	// that no longer exists.
	t.Run("RenameAnnouncesEveryFaceDefaultFirst", func(t *testing.T) {
		obs := &faceRecorder{}
		s := f(t, obs)
		require.NoError(t, s.CreateEntity(ctx(), entity.New("PAGE-1", "page")))
		draft := entity.New("PAGE-1", "page")
		draft.Face = face(t, "draft")
		require.NoError(t, s.CreateEntity(ctx(), draft))

		_, err := s.RenameEntity(ctx(), "PAGE-1", "PAGE-2")
		require.NoError(t, err)
		assert.Equal(t, []string{"PAGE-1->PAGE-2|", "PAGE-1->PAGE-2|draft"}, obs.renames)
		assert.Empty(t, obs.deletes, "a rename is announced as a rename, not as delete+put")
	})
}
