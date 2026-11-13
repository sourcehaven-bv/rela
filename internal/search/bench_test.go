package search_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/search/bleveindex"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// BenchmarkSearch pins the full-text search path the data-entry
// _search endpoint and the MCP search_entities tool sit on
// (TKT-9Y4ZWS): a bleve text query plus store hydration of the hits,
// with a 20-hit limit, at 1k and 5k entities. The size labels are what
// the weekly scale run (tools/scalerun) compares with the previous run, so
// keep them stable.
func BenchmarkSearch(b *testing.B) {
	// Not 10k: seeding indexes one entity at a time and takes about 40 s
	// there, six times over in the scale run.
	for _, n := range []int{1_000, 5_000} {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) { benchSearch(b, n) })
	}
}

func benchSearch(b *testing.B, n int) {
	b.Helper()
	idx, err := bleveindex.NewMem()
	if err != nil {
		b.Fatal(err)
	}
	defer func() { _ = idx.Close() }()

	st := memstore.New(memstore.WithObserver(idx))
	ctx := context.Background()
	for i := range n {
		e := entity.New(fmt.Sprintf("TKT-%05d", i), "ticket")
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
		for _, err := range s.Search(ctx, search.Query{Text: "login", Limit: 20, World: store.TrivialScope()}) {
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
