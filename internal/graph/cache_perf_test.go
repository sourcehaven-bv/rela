package graph

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/model"
)

// BenchmarkSaveCache benchmarks saving the graph to cache
func BenchmarkSaveCache(b *testing.B) {
	for _, size := range []int{100, 1000, 10000} {
		b.Run(fmt.Sprintf("nodes=%d", size), func(b *testing.B) {
			g := generateTestGraph(size, 1.0)
			tmpDir := b.TempDir()
			cachePath := filepath.Join(tmpDir, "cache.json")

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				err := g.SaveCache(cachePath)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkLoadCache benchmarks loading the graph from cache
func BenchmarkLoadCache(b *testing.B) {
	for _, size := range []int{100, 1000, 10000} {
		b.Run(fmt.Sprintf("nodes=%d", size), func(b *testing.B) {
			// Create and save a graph
			g := generateTestGraph(size, 1.0)
			tmpDir := b.TempDir()
			cachePath := filepath.Join(tmpDir, "cache.json")

			if err := g.SaveCache(cachePath); err != nil {
				b.Fatal(err)
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				newGraph := New()
				err := newGraph.LoadCache(cachePath)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkSaveCacheVsSyncFromFiles compares cache loading vs full sync
// This demonstrates the performance benefit of the cache
func BenchmarkSaveCacheVsSyncFromFiles(b *testing.B) {
	for _, size := range []int{100, 1000} {
		b.Run(fmt.Sprintf("nodes=%d/cache", size), func(b *testing.B) {
			g := generateTestGraph(size, 1.0)
			tmpDir := b.TempDir()
			cachePath := filepath.Join(tmpDir, "cache.json")

			if err := g.SaveCache(cachePath); err != nil {
				b.Fatal(err)
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				newGraph := New()
				_ = newGraph.LoadCache(cachePath)
			}
		})
	}
}

// BenchmarkCacheWithLargeProperties tests cache with entities containing many properties
func BenchmarkCacheWithLargeProperties(b *testing.B) {
	for _, numProps := range []int{5, 20, 50} {
		b.Run(fmt.Sprintf("properties=%d", numProps), func(b *testing.B) {
			g := New()

			// Create entities with many properties
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

			tmpDir := b.TempDir()
			cachePath := filepath.Join(tmpDir, "cache.json")

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				_ = g.SaveCache(cachePath)
				newGraph := New()
				_ = newGraph.LoadCache(cachePath)
			}
		})
	}
}

// BenchmarkCacheRebuildAdjacency benchmarks the adjacency map rebuild during load
func BenchmarkCacheRebuildAdjacency(b *testing.B) {
	for _, numEdges := range []int{100, 1000, 10000} {
		b.Run(fmt.Sprintf("edges=%d", numEdges), func(b *testing.B) {
			g := New()

			// Create nodes
			for i := 0; i < 100; i++ {
				g.AddNode(&model.Entity{
					ID:   fmt.Sprintf("ENT-%03d", i),
					Type: "entity",
				})
			}

			// Create edges
			for i := 0; i < numEdges; i++ {
				g.AddEdge(&model.Relation{
					From: fmt.Sprintf("ENT-%03d", i%100),
					To:   fmt.Sprintf("ENT-%03d", (i+1)%100),
					Type: "link",
				})
			}

			tmpDir := b.TempDir()
			cachePath := filepath.Join(tmpDir, "cache.json")

			if err := g.SaveCache(cachePath); err != nil {
				b.Fatal(err)
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				newGraph := New()
				_ = newGraph.LoadCache(cachePath)
			}
		})
	}
}
