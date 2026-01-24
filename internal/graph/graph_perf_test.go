package graph

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/model"
)

// generateTestGraph creates a graph with n entities and approximately n*relationsPerEntity relations
func generateTestGraph(n int, relationsPerEntity float64) *Graph {
	g := New()

	// Define entity types
	types := []string{"requirement", "decision", "solution", "component"}
	numRelations := int(float64(n) * relationsPerEntity)

	// Create n entities
	for i := 0; i < n; i++ {
		entityType := types[i%len(types)]
		prefix := map[string]string{
			"requirement": "REQ-",
			"decision":    "DEC-",
			"solution":    "SOL-",
			"component":   "COMP-",
		}[entityType]

		entity := &model.Entity{
			ID:   fmt.Sprintf("%s%03d", prefix, i),
			Type: entityType,
			Properties: map[string]interface{}{
				"title":  fmt.Sprintf("Test %s %d", entityType, i),
				"status": "draft",
			},
		}
		g.AddNode(entity)
	}

	// Create relations between random entities
	ids := g.AllIDs()
	relationTypes := []string{"addresses", "implements", "realizes", "dependsOn"}

	for i := 0; i < numRelations; i++ {
		from := ids[rand.Intn(len(ids))]
		to := ids[rand.Intn(len(ids))]
		if from == to {
			continue
		}
		relType := relationTypes[rand.Intn(len(relationTypes))]
		g.AddEdge(&model.Relation{
			From: from,
			Type: relType,
			To:   to,
		})
	}

	return g
}

// BenchmarkGraphAddNode benchmarks adding nodes to the graph
func BenchmarkGraphAddNode(b *testing.B) {
	for _, size := range []int{100, 1000, 10000} {
		b.Run(fmt.Sprintf("size=%d", size), func(b *testing.B) {
			entities := make([]*model.Entity, size)
			for i := 0; i < size; i++ {
				entities[i] = &model.Entity{
					ID:   fmt.Sprintf("ENT-%03d", i),
					Type: "requirement",
					Properties: map[string]interface{}{
						"title": fmt.Sprintf("Entity %d", i),
					},
				}
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				g := New()
				for _, e := range entities {
					g.AddNode(e)
				}
			}
		})
	}
}

// BenchmarkGraphAddEdge benchmarks adding edges to an existing graph
func BenchmarkGraphAddEdge(b *testing.B) {
	for _, size := range []int{100, 1000, 10000} {
		b.Run(fmt.Sprintf("edges=%d", size), func(b *testing.B) {
			// Create a graph with nodes
			g := New()
			for i := 0; i < size; i++ {
				g.AddNode(&model.Entity{
					ID:   fmt.Sprintf("ENT-%03d", i),
					Type: "entity",
				})
			}

			// Prepare edges
			edges := make([]*model.Relation, size)
			ids := g.AllIDs()
			for i := 0; i < size; i++ {
				edges[i] = &model.Relation{
					From: ids[i%len(ids)],
					To:   ids[(i+1)%len(ids)],
					Type: "link",
				}
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				testGraph := New()
				for _, e := range g.AllNodes() {
					testGraph.AddNode(e)
				}
				for _, edge := range edges {
					testGraph.AddEdge(edge)
				}
			}
		})
	}
}

// BenchmarkGraphNodesByType benchmarks filtering nodes by type
// This is O(n) - iterates all nodes
func BenchmarkGraphNodesByType(b *testing.B) {
	for _, size := range []int{100, 1000, 10000} {
		b.Run(fmt.Sprintf("nodes=%d", size), func(b *testing.B) {
			g := generateTestGraph(size, 0.5)

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				_ = g.NodesByType("requirement")
			}
		})
	}
}

// BenchmarkGraphGetEdge benchmarks looking up a specific edge
// This is O(e) - linear scan through edges
func BenchmarkGraphGetEdge(b *testing.B) {
	for _, numEdges := range []int{100, 1000, 10000} {
		b.Run(fmt.Sprintf("edges=%d", numEdges), func(b *testing.B) {
			g := New()
			// Create nodes
			for i := 0; i < 100; i++ {
				g.AddNode(&model.Entity{ID: fmt.Sprintf("ENT-%03d", i), Type: "entity"})
			}
			// Create edges
			for i := 0; i < numEdges; i++ {
				g.AddEdge(&model.Relation{
					From: fmt.Sprintf("ENT-%03d", i%100),
					To:   fmt.Sprintf("ENT-%03d", (i+1)%100),
					Type: fmt.Sprintf("rel-%d", i),
				})
			}

			// Look for the last edge (worst case)
			targetFrom := fmt.Sprintf("ENT-%03d", (numEdges-1)%100)
			targetTo := fmt.Sprintf("ENT-%03d", numEdges%100)
			targetType := fmt.Sprintf("rel-%d", numEdges-1)

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				_, _ = g.GetEdge(targetFrom, targetType, targetTo)
			}
		})
	}
}

// BenchmarkGraphRemoveNode benchmarks node removal with adjacency rebuild
// This is O(e) for edge filtering + O(e) for adjacency rebuild
func BenchmarkGraphRemoveNode(b *testing.B) {
	for _, size := range []int{100, 1000, 10000} {
		b.Run(fmt.Sprintf("nodes=%d", size), func(b *testing.B) {
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				b.StopTimer()
				g := generateTestGraph(size, 1.0)
				targetID := fmt.Sprintf("REQ-%03d", 0)
				b.StartTimer()

				g.RemoveNode(targetID)
			}
		})
	}
}

// BenchmarkGraphRemoveEdge benchmarks edge removal with adjacency rebuild
func BenchmarkGraphRemoveEdge(b *testing.B) {
	for _, numEdges := range []int{100, 1000, 10000} {
		b.Run(fmt.Sprintf("edges=%d", numEdges), func(b *testing.B) {
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				b.StopTimer()
				g := New()
				for j := 0; j < 100; j++ {
					g.AddNode(&model.Entity{ID: fmt.Sprintf("ENT-%03d", j), Type: "entity"})
				}
				for j := 0; j < numEdges; j++ {
					g.AddEdge(&model.Relation{
						From: fmt.Sprintf("ENT-%03d", j%100),
						To:   fmt.Sprintf("ENT-%03d", (j+1)%100),
						Type: "link",
					})
				}
				b.StartTimer()

				g.RemoveEdge(fmt.Sprintf("ENT-%03d", 0), "link", fmt.Sprintf("ENT-%03d", 1))
			}
		})
	}
}

// BenchmarkGraphAllNodes benchmarks retrieving all nodes
func BenchmarkGraphAllNodes(b *testing.B) {
	for _, size := range []int{100, 1000, 10000} {
		b.Run(fmt.Sprintf("nodes=%d", size), func(b *testing.B) {
			g := generateTestGraph(size, 0.5)

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				_ = g.AllNodes()
			}
		})
	}
}

// BenchmarkGraphTraceFrom benchmarks tracing from a node
// This is O(v+e) in the worst case for full traversal
func BenchmarkGraphTraceFrom(b *testing.B) {
	for _, size := range []int{100, 1000, 10000} {
		b.Run(fmt.Sprintf("nodes=%d", size), func(b *testing.B) {
			g := generateTestGraph(size, 1.0)
			ids := g.AllIDs()
			startID := ids[0]

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				_ = g.TraceFrom(startID, 0) // maxDepth=0 means unlimited
			}
		})
	}
}

// BenchmarkGraphTraceFromLimited benchmarks tracing with depth limit
func BenchmarkGraphTraceFromLimited(b *testing.B) {
	for _, size := range []int{100, 1000, 10000} {
		b.Run(fmt.Sprintf("nodes=%d", size), func(b *testing.B) {
			g := generateTestGraph(size, 1.0)
			ids := g.AllIDs()
			startID := ids[0]

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				_ = g.TraceFrom(startID, 3) // limit to depth 3
			}
		})
	}
}

// BenchmarkGraphFindPath benchmarks BFS path finding
func BenchmarkGraphFindPath(b *testing.B) {
	for _, size := range []int{100, 1000, 10000} {
		b.Run(fmt.Sprintf("nodes=%d", size), func(b *testing.B) {
			g := generateTestGraph(size, 1.0)
			ids := g.AllIDs()
			fromID := ids[0]
			toID := ids[len(ids)-1]

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				_ = g.FindPath(fromID, toID)
			}
		})
	}
}

// BenchmarkGraphFindOrphans benchmarks orphan detection
// This is O(n) - iterates all nodes
func BenchmarkGraphFindOrphans(b *testing.B) {
	for _, size := range []int{100, 1000, 10000} {
		b.Run(fmt.Sprintf("nodes=%d", size), func(b *testing.B) {
			g := generateTestGraph(size, 0.5)

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				_ = g.FindOrphans()
			}
		})
	}
}

// BenchmarkGraphFindClusters benchmarks connected component detection
// This is O(v+e) using BFS
func BenchmarkGraphFindClusters(b *testing.B) {
	for _, size := range []int{100, 1000, 10000} {
		b.Run(fmt.Sprintf("nodes=%d", size), func(b *testing.B) {
			g := generateTestGraph(size, 0.5)

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				_ = g.FindClusters()
			}
		})
	}
}

// BenchmarkGraphRelationsOfType benchmarks filtering relations by type
// This is O(e) - linear scan through edges
func BenchmarkGraphRelationsOfType(b *testing.B) {
	for _, numEdges := range []int{100, 1000, 10000} {
		b.Run(fmt.Sprintf("edges=%d", numEdges), func(b *testing.B) {
			g := generateTestGraph(100, float64(numEdges)/100.0)

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				_ = g.RelationsOfType("addresses")
			}
		})
	}
}
