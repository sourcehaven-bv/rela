package markdown

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
)

// generateEntityContent creates markdown content for testing
func generateEntityContent(id, entityType, title string) string {
	return fmt.Sprintf(`---
id: %s
type: %s
title: %s
status: draft
description: This is a test entity for performance benchmarking. It contains some additional text to make the content more realistic.
---

## Overview

This is the body content of the entity. It contains multiple paragraphs of text to simulate realistic entity content.

### Details

- Item 1: Some detailed information
- Item 2: More detailed information
- Item 3: Even more details

## References

See related entities for more information.
`, id, entityType, title)
}

// generateRelationContent creates relation markdown content
func generateRelationContent(from, relType, to string) string {
	return fmt.Sprintf(`---
from: %s
relation: %s
to: %s
---
`, from, relType, to)
}

// BenchmarkParseDocument benchmarks YAML frontmatter parsing
func BenchmarkParseDocument(b *testing.B) {
	content := generateEntityContent("REQ-001", "requirement", "Test Requirement")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := ParseDocument(content)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkParseDocumentLargeContent benchmarks parsing with larger content
func BenchmarkParseDocumentLargeContent(b *testing.B) {
	for _, contentSize := range []int{100, 1000, 10000} {
		b.Run(fmt.Sprintf("content_bytes=%d", contentSize), func(b *testing.B) {
			// Generate content of approximately specified size
			baseContent := generateEntityContent("REQ-001", "requirement", "Test Requirement")
			for len(baseContent) < contentSize {
				baseContent += "\nAdditional content to increase size. "
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				_, err := ParseDocument(baseContent)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkFormatDocument benchmarks document formatting
func BenchmarkFormatDocument(b *testing.B) {
	frontmatter := map[string]interface{}{
		"id":          "REQ-001",
		"type":        "requirement",
		"title":       "Test Requirement",
		"status":      "draft",
		"description": "A test description",
	}
	content := "## Overview\n\nThis is the body content."

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := FormatDocument(frontmatter, content)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkReadEntity benchmarks reading an entity from file
func BenchmarkReadEntity(b *testing.B) {
	// Create a temporary file
	tmpDir := b.TempDir()
	content := generateEntityContent("REQ-001", "requirement", "Test Requirement")
	filePath := filepath.Join(tmpDir, "REQ-001.md")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		b.Fatal(err)
	}

	meta := metamodel.DefaultMetamodel()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := ReadEntity(filePath, meta)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkWriteEntity benchmarks writing an entity to file
func BenchmarkWriteEntity(b *testing.B) {
	tmpDir := b.TempDir()
	entity := &model.Entity{
		ID:   "REQ-001",
		Type: "requirement",
		Properties: map[string]interface{}{
			"title":       "Test Requirement",
			"status":      "draft",
			"description": "A test description",
		},
		Content: "## Overview\n\nThis is the body content.",
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		filePath := filepath.Join(tmpDir, fmt.Sprintf("REQ-%03d.md", i))
		err := WriteEntity(entity, filePath)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkListEntityFiles benchmarks directory traversal for entity files
func BenchmarkListEntityFiles(b *testing.B) {
	for _, numFiles := range []int{10, 100, 1000} {
		b.Run(fmt.Sprintf("files=%d", numFiles), func(b *testing.B) {
			// Create temp directory with subdirectories and files
			tmpDir := b.TempDir()

			types := []string{"requirements", "decisions", "solutions", "components"}
			filesPerType := numFiles / len(types)

			for _, typeName := range types {
				typeDir := filepath.Join(tmpDir, typeName)
				if err := os.MkdirAll(typeDir, 0755); err != nil {
					b.Fatal(err)
				}

				for i := 0; i < filesPerType; i++ {
					content := generateEntityContent(
						fmt.Sprintf("%s-%03d", typeName[:3], i),
						typeName[:len(typeName)-1],
						fmt.Sprintf("Test %d", i),
					)
					filePath := filepath.Join(typeDir, fmt.Sprintf("%s-%03d.md", typeName[:3], i))
					if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
						b.Fatal(err)
					}
				}
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				_, err := ListEntityFiles(tmpDir)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkLoadAllEntities benchmarks loading all entities from disk
func BenchmarkLoadAllEntities(b *testing.B) {
	for _, numFiles := range []int{10, 100, 1000} {
		b.Run(fmt.Sprintf("files=%d", numFiles), func(b *testing.B) {
			tmpDir := b.TempDir()
			meta := metamodel.DefaultMetamodel()

			// Create temp files
			types := []string{"requirements", "decisions", "solutions", "components"}
			filesPerType := numFiles / len(types)

			for _, typeName := range types {
				typeDir := filepath.Join(tmpDir, typeName)
				if err := os.MkdirAll(typeDir, 0755); err != nil {
					b.Fatal(err)
				}

				for i := 0; i < filesPerType; i++ {
					prefix := map[string]string{
						"requirements": "REQ",
						"decisions":    "DEC",
						"solutions":    "SOL",
						"components":   "COMP",
					}[typeName]
					entityType := typeName[:len(typeName)-1]

					content := generateEntityContent(
						fmt.Sprintf("%s-%03d", prefix, i),
						entityType,
						fmt.Sprintf("Test %s %d", entityType, i),
					)
					filePath := filepath.Join(typeDir, fmt.Sprintf("%s-%03d.md", prefix, i))
					if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
						b.Fatal(err)
					}
				}
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				_, err := LoadAllEntities(tmpDir, meta)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkLoadAllRelations benchmarks loading all relations from disk
func BenchmarkLoadAllRelations(b *testing.B) {
	for _, numFiles := range []int{10, 100, 1000} {
		b.Run(fmt.Sprintf("files=%d", numFiles), func(b *testing.B) {
			tmpDir := b.TempDir()

			// Create relation files
			for i := 0; i < numFiles; i++ {
				from := fmt.Sprintf("REQ-%03d", i%100)
				to := fmt.Sprintf("DEC-%03d", i%100)
				content := generateRelationContent(from, "addresses", to)
				filename := RelationFilename(from, "addresses", to)
				filePath := filepath.Join(tmpDir, filename)
				if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
					b.Fatal(err)
				}
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				_, err := LoadAllRelations(tmpDir)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkSplitFrontmatter benchmarks the frontmatter splitting operation
func BenchmarkSplitFrontmatter(b *testing.B) {
	content := generateEntityContent("REQ-001", "requirement", "Test Requirement")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _, err := splitFrontmatter(content)
		if err != nil {
			b.Fatal(err)
		}
	}
}
