package search_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/search/bleveindex"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// BenchmarkSearch pins the full-text search path the data-entry
// _search endpoint and the MCP search_entities tool sit on
// (TKT-9Y4ZWS): a bleve text query plus store hydration of the hits,
// against a 1000-entity index with a 20-hit limit.
func BenchmarkSearch(b *testing.B) {
	idx, err := bleveindex.NewMem()
	if err != nil {
		b.Fatal(err)
	}
	defer func() { _ = idx.Close() }()

	st := memstore.New(memstore.WithObserver(idx))
	ctx := context.Background()
	for i := range 1000 {
		e := entity.New(fmt.Sprintf("TKT-%04d", i), "ticket")
		// Vary the text so the query has both matches and misses to rank.
		e.SetString("title", fmt.Sprintf("ticket %d: %s in widget %d",
			i, []string{"login failure", "render glitch", "sync stall", "crash report"}[i%4], i%37))
		if err := st.CreateEntity(ctx, e); err != nil {
			b.Fatalf("seed %d: %v", i, err)
		}
	}

	s := search.New(st, idx)

	b.ReportAllocs()
	for b.Loop() {
		hits := 0
		for _, err := range s.Search(ctx, search.Query{Text: "login", Limit: 20}) {
			if err != nil {
				b.Fatal(err)
			}
			hits++
		}
		if hits == 0 {
			b.Fatal("query matched nothing — fixture broken")
		}
	}
}
