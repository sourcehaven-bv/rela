package storetest

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// RunValidationTests runs input validation conformance tests.
func RunValidationTests(t *testing.T, f Factory) {
	t.Run("CreateEntityRejectsInvalidIDs", func(t *testing.T) {
		s := f(t)

		invalid := []struct {
			id   string
			want string
		}{
			{"", "empty"},
			{"foo/bar", "path separator"},
			{"foo\\bar", "path separator"},
			{"foo\x00bar", "control character"},
			{"foo\nbar", "control character"},
			{"foo\tbar", "control character"},
			{"foo\x7fbar", "control character"},
			{"a--b", "consecutive dashes"},
		}
		for _, tc := range invalid {
			err := s.CreateEntity(ctx(), entity.New(tc.id, "t"))
			assert.Errorf(t, err, "id %q should be rejected", tc.id)
			if err != nil {
				assert.Containsf(t, err.Error(), tc.want,
					"error for id %q should mention %q", tc.id, tc.want)
			}
		}
	})

	t.Run("RelationKeyRejectsDoubleDash", func(t *testing.T) {
		s := f(t)

		err := s.CreateEntity(ctx(), entity.New("A--B", "t"))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "consecutive dashes")

		require.NoError(t, s.CreateEntity(ctx(), entity.New("A-B", "t")))
		require.NoError(t, s.CreateEntity(ctx(), entity.New("C-D", "t")))

		_, err = s.CreateRelation(ctx(), "A-B", "req--ires", "C-D", nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "consecutive dashes")

		_, err = s.CreateRelation(ctx(), "A-B", "requires", "C-D", nil)
		require.NoError(t, err)
	})

	t.Run("RenameKeyCollapseDeterministic", func(t *testing.T) {
		s := f(t)

		require.NoError(t, s.CreateEntity(ctx(), entity.New("A", "t")))
		require.NoError(t, s.CreateEntity(ctx(), entity.New("C", "t")))
		require.NoError(t, s.CreateEntity(ctx(), entity.New("X", "t")))

		_, err := s.CreateRelation(ctx(), "A", "requires", "X",
			&store.RelationData{Content: "from-A"})
		require.NoError(t, err)
		_, err = s.CreateRelation(ctx(), "C", "requires", "X",
			&store.RelationData{Content: "from-C"})
		require.NoError(t, err)

		_, err = s.RenameEntity(ctx(), "A", "C")
		assert.ErrorIs(t, err, store.ErrConflict)
	})

	// Case-variant IDs are ONE identity, in every backend (BUG-3RCWNS).
	//
	// This lives in the shared conformance suite rather than in fsstore's own
	// tests on purpose. fsstore on a case-insensitive filesystem (macOS,
	// Windows) folds "abc" and "ABC" onto one file, while memstore and pgstore
	// (id TEXT COLLATE "C") keep them as two rows. Entities move between
	// backends via migration and `rela sync`, so a project holding both would
	// silently lose one on import. The backends must agree on identity, and
	// only a shared test can enforce that.
	//
	// Note this cannot be satisfied by a filesystem accident: the byte-exact
	// backends must actively fold case to pass.
	t.Run("CreateRejectsCaseVariantID", func(t *testing.T) {
		s := f(t)

		require.NoError(t, s.CreateEntity(ctx(), entity.New("abc", "t")))

		err := s.CreateEntity(ctx(), entity.New("ABC", "t"))
		assert.ErrorIsf(t, err, store.ErrConflict,
			"creating \"ABC\" while \"abc\" exists must conflict, not silently overwrite")

		// The original must be intact and still reachable under its own ID.
		got, err := s.GetEntity(ctx(), "abc")
		require.NoError(t, err)
		assert.Equal(t, "abc", got.ID)
	})

	t.Run("RenameRejectsCaseVariantID", func(t *testing.T) {
		s := f(t)

		require.NoError(t, s.CreateEntity(ctx(), entity.New("abc", "t")))
		require.NoError(t, s.CreateEntity(ctx(), entity.New("other", "t")))

		_, err := s.RenameEntity(ctx(), "other", "ABC")
		assert.ErrorIsf(t, err, store.ErrConflict,
			"renaming to \"ABC\" while \"abc\" exists must conflict")
	})

	// Renaming an entity to a different casing OF ITSELF is legitimate
	// (abc -> ABC is one entity changing its own display casing), so the
	// case-folded conflict check must not reject it as a self-collision.
	t.Run("RenameToOwnCaseVariantIsAllowed", func(t *testing.T) {
		s := f(t)

		require.NoError(t, s.CreateEntity(ctx(), entity.New("abc", "t")))

		_, err := s.RenameEntity(ctx(), "abc", "ABC")
		require.NoError(t, err, "an entity may change its own casing")

		got, err := s.GetEntity(ctx(), "ABC")
		require.NoError(t, err)
		assert.Equal(t, "ABC", got.ID)
	})

	// Invalid UTF-8 in a property value must be REFUSED, identically, by
	// every backend (BUG-X7ICNM). Before this rule fsstore refused it (YAML
	// cannot represent it), pgstore and sqlitestore silently substituted
	// U+FFFD and reported success, and memstore kept the bytes. The rule is
	// storeutil.ValidateProperties; these cases pin that each write path
	// actually calls it, at the nesting the store fuzz target generates.
	// A property NAME beginning with git's conflict marker must round-trip on
	// every backend (BUG-TOXQAA / issue #993). fsstore wrote it as a YAML
	// mapping key at column 0, so the file it had just written scanned as an
	// unresolved merge and every later read refused it — the entity was
	// unreadable and silently absent from the validator and the search index,
	// while memstore and pgstore round-tripped it fine. The fix is in the
	// markdown emitter (the key is quoted), not a new validity rule: the value
	// is representable, so a serialization limit must not decide what an
	// entity may contain. These cases pin that the backends agree.
	t.Run("RoundTripsConflictMarkerPropertyName", func(t *testing.T) {
		s := f(t)

		for i, name := range []string{"<<<<<<<", "<<<<<<< HEAD", "x<<<<<<<"} {
			id := fmt.Sprintf("E-cm%d", i)
			e := entity.New(id, "t")
			e.Properties[name] = "v"
			require.NoErrorf(t, s.CreateEntity(ctx(), e), "create with property %q", name)

			got, err := s.GetEntity(ctx(), id)
			require.NoErrorf(t, err, "read back entity with property %q", name)
			assert.Equalf(t, "v", got.Properties[name], "property %q lost its value", name)
		}
	})

	t.Run("RejectsInvalidUTF8Properties", func(t *testing.T) {
		const bad = "\n\xc80" // the fuzzer's original payload
		s := f(t)

		shapes := map[string]any{
			"string":     bad,
			"slice":      []string{"ok", bad},
			"nested map": map[string]any{"v": bad},
		}
		for name, val := range shapes {
			e := entity.New("E-"+strings.ReplaceAll(name, " ", "-"), "t")
			e.Properties["p"] = val
			err := s.CreateEntity(ctx(), e)
			require.Errorf(t, err, "create with %s invalid UTF-8 must fail", name)
			assert.Contains(t, err.Error(), "invalid UTF-8")
			_, err = s.GetEntity(ctx(), e.ID)
			assert.ErrorIs(t, err, store.ErrNotFound, "a refused create must persist nothing")
		}

		good := entity.New("E-good", "t")
		good.SetString("p", "héllo ☃")
		require.NoError(t, s.CreateEntity(ctx(), good))
		got, err := s.GetEntity(ctx(), "E-good")
		require.NoError(t, err)
		assert.Equal(t, "héllo ☃", got.GetString("p"), "valid non-ASCII must round-trip untouched")

		upd := got.Clone()
		upd.SetString("p", bad)
		err = s.UpdateEntity(ctx(), upd)
		require.Error(t, err, "update with invalid UTF-8 must fail")
		assert.Contains(t, err.Error(), "invalid UTF-8")
		got, err = s.GetEntity(ctx(), "E-good")
		require.NoError(t, err)
		assert.Equal(t, "héllo ☃", got.GetString("p"), "a refused update must leave the stored value alone")

		require.NoError(t, s.CreateEntity(ctx(), entity.New("E-other", "t")))
		_, err = s.CreateRelation(ctx(), "E-good", "rel", "E-other",
			&store.RelationData{Properties: map[string]any{"p": bad}})
		require.Error(t, err, "relation create with invalid UTF-8 must fail")
		assert.Contains(t, err.Error(), "invalid UTF-8")
		_, err = s.GetRelation(ctx(), "E-good", "rel", "E-other")
		assert.ErrorIs(t, err, store.ErrNotFound)

		_, err = s.CreateRelation(ctx(), "E-good", "rel", "E-other",
			&store.RelationData{Properties: map[string]any{"p": "ok"}})
		require.NoError(t, err)
		_, err = s.UpdateRelation(ctx(), "E-good", "rel", "E-other",
			store.RelationData{Properties: map[string]any{"p": bad}})
		require.Error(t, err, "relation update with invalid UTF-8 must fail")
		assert.Contains(t, err.Error(), "invalid UTF-8")
		r, err := s.GetRelation(ctx(), "E-good", "rel", "E-other")
		require.NoError(t, err)
		assert.Equal(t, "ok", r.Properties["p"], "a refused relation update must leave the stored value alone")
	})
}
