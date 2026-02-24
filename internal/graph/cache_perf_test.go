package graph

import (
	"fmt"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/model"
)

// BenchmarkSnapshot benchmarks taking a graph snapshot
func BenchmarkSnapshot(b *testing.B) {
	for _, size := range []int{100, 1000, 10000} {
		b.Run(fmt.Sprintf("nodes=%d", size), func(b *testing.B) {
			g := generateTestGraph(size, 1.0)

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				_ = g.Snapshot()
			}
		})
	}
}

// BenchmarkRestore benchmarks restoring a graph from cache data
func BenchmarkRestore(b *testing.B) {
	for _, size := range []int{100, 1000, 10000} {
		b.Run(fmt.Sprintf("nodes=%d", size), func(b *testing.B) {
			g := generateTestGraph(size, 1.0)
			snap := g.Snapshot()

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				newGraph := New()
				newGraph.Restore(snap)
			}
		})
	}
}

// BenchmarkSnapshotRestore benchmarks a full round-trip
func BenchmarkSnapshotRestore(b *testing.B) {
	for _, size := range []int{100, 1000} {
		b.Run(fmt.Sprintf("nodes=%d", size), func(b *testing.B) {
			g := generateTestGraph(size, 1.0)

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				snap := g.Snapshot()
				newGraph := New()
				newGraph.Restore(snap)
			}
		})
	}
}

// BenchmarkRestoreWithLargeProperties tests restore with entities containing many properties
func BenchmarkRestoreWithLargeProperties(b *testing.B) {
	for _, numProps := range []int{5, 20, 50} {
		b.Run(fmt.Sprintf("properties=%d", numProps), func(b *testing.B) {
			g := New()

			for i := 0; i < 1000; i++ {
				props := make(map[string]interface{})
				for j := 0; j < numProps; j++ {
					props[fmt.Sprintf("property%d", j)] = fmt.Sprintf("value%d_%d", i, j)
				}
				props["title"] = fmt.Sprintf("Entity %d", i)
				props["status"] = "draft"

				g.AddNode(&model.Entity{
					ID:         fmt.Sprintf("ENT-%03d", i),
					Type:       "entity",
					Properties: props,
				})
			}

			snap := g.Snapshot()

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				newGraph := New()
				newGraph.Restore(snap)
			}
		})
	}
}

// BenchmarkRestoreRebuildAdjacency benchmarks the adjacency map rebuild during restore
func BenchmarkRestoreRebuildAdjacency(b *testing.B) {
	for _, numEdges := range []int{100, 1000, 10000} {
		b.Run(fmt.Sprintf("edges=%d", numEdges), func(b *testing.B) {
			g := New()

			for i := 0; i < 100; i++ {
				g.AddNode(&model.Entity{
					ID:   fmt.Sprintf("ENT-%03d", i),
					Type: "entity",
				})
			}

			for i := 0; i < numEdges; i++ {
				g.AddEdge(&model.Relation{
					From: fmt.Sprintf("ENT-%03d", i%100),
					To:   fmt.Sprintf("ENT-%03d", (i+1)%100),
					Type: "link",
				})
			}

			snap := g.Snapshot()

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				newGraph := New()
				newGraph.Restore(snap)
			}
		})
	}
}
