package pgstore_test

import (
	"fmt"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
)

// BenchmarkSearchBackend measures the in-database full-text search the
// postgres build's _search endpoint sits on: a trigram substring match
// ranked by similarity, 20 hits, over 1k and 10k entities of which a
// quarter match. The weekly scale run (tools/scalerun) compares it with the
// previous run, so a size label stays stable once added.
func BenchmarkSearchBackend(b *testing.B) {
	for _, n := range []int{1_000, 10_000} {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			_, pool := benchSetup(b, n)
			sb := pgstore.NewSearchBackend(pool)
			b.ReportAllocs()
			for b.Loop() {
				hits, err := sb.Search("login", 20, store.TrivialScope())
				if err != nil {
					b.Fatal(err)
				}
				if len(hits) == 0 {
					b.Fatal("query matched nothing; setup is broken")
				}
			}
		})
	}
}
