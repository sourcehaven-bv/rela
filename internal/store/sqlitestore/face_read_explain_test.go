package sqlitestore_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/sqlitestore"
	"github.com/Sourcehaven-BV/rela/internal/store/storeutil"
)

// Every per-face and family read is served by an index (TKT-KQXVF7, schema
// v8).
//
// The fixture mixes a faceless type (task: 5,000 rows at the implicit face)
// with a faced type (policy: 2,500 families at draft and published, no row at
// the implicit face). Before v8 a type page used entities_type_idx (type) and
// sorted the whole type, and HighestID's LIKE scanned the table. No ANALYZE
// runs, as in production.
func TestFaceReadExplain(t *testing.T) {
	s := open(t)
	seed(t, s, func(v store.Store) {
		for i := range 5000 {
			mustCreate(t, v, entity.New(fmt.Sprintf("TASK-%06d", i), "task"))
		}
		for i := range 2500 {
			for _, face := range []entity.Face{"draft", "published"} {
				e := entity.New(fmt.Sprintf("POL-%06d", i), "policy")
				e.Face = face
				mustCreate(t, v, e)
			}
		}
	})
	ctx := context.Background()

	published := store.NewWorldScope(map[string]store.TypeResolution{
		"policy": {Chain: []entity.Face{"published", "draft"}, Fallback: store.FallbackExclude},
	})
	family := make([]string, 50)
	for i := range family {
		family[i] = fmt.Sprintf("POL-%06d", i*37)
	}

	pageIndex := "USING INDEX entities_type_id_face_idx"
	// The world page sorts twice. The first sort ranks the faces of each
	// family: "LAST 2 TERMS" means the index still supplies id order and the
	// sort covers one family at a time. The second orders the primes. SQLite
	// cannot see that ROW_NUMBER's output is already in id order, so that
	// sort still covers every prime of the type before LIMIT. pgstore bounds
	// the pick with an inner LIMIT; SQLite would need the pick rewritten
	// without a window.
	worldSorts := []string{"USE TEMP B-TREE FOR LAST 2 TERMS OF ORDER BY", "USE TEMP B-TREE FOR ORDER BY"}
	for _, tc := range []struct {
		name  string
		plan  func() (string, error)
		index string
		// sorts lists the plan's temp b-trees, exactly. Only the world page
		// and HighestID's de-duplication have any.
		sorts []string
	}{
		{"default-face page", func() (string, error) {
			return s.ExplainEntityPage(ctx, store.EntityQuery{Type: "task", Limit: 100, Faces: store.InWorld(store.DefaultWorld())})
		}, pageIndex, nil},
		{"all-faces page", func() (string, error) {
			return s.ExplainEntityPage(ctx, store.EntityQuery{Type: "policy", Faces: store.AllFaces(), Limit: 100})
		}, pageIndex, nil},
		{"world page", func() (string, error) {
			return s.ExplainEntityPage(ctx, store.EntityQuery{Type: "policy", Faces: store.InWorld(published), Limit: 100})
		}, pageIndex, worldSorts},
		{"world page after a cursor", func() (string, error) {
			return s.ExplainEntityPage(ctx, store.EntityQuery{
				Type: "policy", Faces: store.InWorld(published), Limit: 100,
				Cursor: storeutil.EncodeCursor("POL-001200@published"),
			})
		}, pageIndex, worldSorts},
		{"explicit-faces page", func() (string, error) {
			return s.ExplainEntityPage(ctx, store.EntityQuery{
				Type: "policy", Faces: store.AllFaces(), FaceIn: []entity.Face{"published"}, Limit: 100,
			})
		}, pageIndex, nil},
		{"family read", func() (string, error) {
			return s.ExplainEntityPage(ctx, store.EntityQuery{IDs: family, Faces: store.AllFaces()})
		}, "USING INDEX sqlite_autoindex_entities_1", nil},
		{"single row", func() (string, error) {
			return s.ExplainGetEntityState(ctx, "POL-000007", "published")
		}, "USING INDEX sqlite_autoindex_entities_1", nil},
		{"highest id", func() (string, error) {
			return s.ExplainHighestID(ctx, "POL")
		}, "USING INDEX entities_id_lower_key", []string{"USE TEMP B-TREE FOR DISTINCT"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			plan, err := tc.plan()
			require.NoError(t, err)
			t.Logf("plan:\n%s", plan)

			require.Contains(t, plan, tc.index, "the read is not served by its index")
			var sorts []string
			for line := range strings.SplitSeq(plan, "\n") {
				require.NotContains(t, line, "SCAN entities", "the read scans the table:\n%s", plan)
				if strings.HasPrefix(line, "USE TEMP B-TREE") {
					sorts = append(sorts, line)
				}
			}
			require.Equal(t, tc.sorts, sorts, "the read sorts more than it must:\n%s", plan)
		})
	}
}

// HighestID folds ASCII case on this backend, as the LIKE it replaced did:
// ids are case-insensitive identities (entities_id_lower_key). storetest
// cannot pin the fold, because the other backends are case-sensitive.
func TestHighestIDFoldsASCIICase(t *testing.T) {
	s := open(t)
	seed(t, s, func(v store.Store) {
		for _, id := range []string{"FEAT-4", "feat-9", "FEATURE-20"} {
			mustCreate(t, v, entity.New(id, "feature"))
		}
	})
	n, err := s.HighestID(context.Background(), "FEAT")
	require.NoError(t, err)
	require.Equal(t, 9, n)
}

func TestBuildHighestIDSQLBounds(t *testing.T) {
	_, args := sqlitestore.BuildHighestIDSQLForTest("FEAT")
	require.Equal(t, []any{"FEAT-", "FEAT."}, args)
}
