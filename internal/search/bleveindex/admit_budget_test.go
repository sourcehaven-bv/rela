package bleveindex

import (
	"fmt"
	"testing"

	"github.com/blevesearch/bleve/v2"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// bleveIndex lets countingIndex embed the interface without its field name
// colliding with the Index method.
type bleveIndex = bleve.Index

// countingIndex counts the queries run against the wrapped bleve index.
type countingIndex struct {
	bleveIndex
	searches int
}

func (c *countingIndex) Search(req *bleve.SearchRequest) (*bleve.SearchResult, error) {
	c.searches++
	return c.bleveIndex.Search(req)
}

// TestSearchAdmitted_QueryBudgetIsHitIndependent pins that an admitting
// search runs the same number of index queries at 10 and at 50 hits: the
// trivial world reads no family, and a scoped world reads every family in
// one batched query rather than one query per hit.
func TestSearchAdmitted_QueryBudgetIsHitIndependent(t *testing.T) {
	scoped := store.NewWorldScope(map[string]store.TypeResolution{
		"policy": {Chain: []entity.Face{"draft", "published"}, Fallback: store.FallbackExclude},
	})
	for _, tc := range []struct {
		name  string
		world store.WorldScope
		faces []entity.Face
		want  int
	}{
		{"trivial world", store.TrivialScope(), []entity.Face{""}, 1},
		{"scoped world", scoped, []entity.Face{"draft", "published"}, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, n := range []int{10, 50} {
				idx, err := NewMem()
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { idx.Close() })
				for i := range n {
					for _, f := range tc.faces {
						e := entity.New(fmt.Sprintf("POL-%d", i), "policy")
						e.Face = f
						e.SetString("title", "alpha "+string(f))
						if perr := idx.EntityPut(e); perr != nil {
							t.Fatal(perr)
						}
					}
				}
				counter := &countingIndex{bleveIndex: idx.index}
				idx.index = counter
				admitted := 0
				hits, err := idx.SearchAdmitted("alpha", 0, tc.world,
					func(c []search.Candidate) ([]search.Candidate, error) {
						admitted += len(c)
						return c, nil
					})
				if err != nil {
					t.Fatal(err)
				}
				if len(hits) != n {
					t.Fatalf("n=%d: %d hits, want %d", n, len(hits), n)
				}
				if want := n * len(tc.faces); admitted != want {
					t.Errorf("n=%d: admitted %d candidates, want %d", n, admitted, want)
				}
				if counter.searches != tc.want {
					t.Errorf("n=%d: %d index queries, want %d", n, counter.searches, tc.want)
				}
			}
		})
	}
}
