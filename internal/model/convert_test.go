package model

import (
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

func TestEntityRoundTrip(t *testing.T) {
	ts := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	legacy := &Entity{
		ID:         "REQ-001",
		Type:       "requirement",
		Properties: map[string]interface{}{"title": "Test", "priority": "high"},
		Content:    "body text",
		FilePath:   "/some/path.md",
		ModTime:    ts,
	}

	domain := EntityToDomain(legacy)

	if domain.ID != legacy.ID {
		t.Errorf("ID: got %q, want %q", domain.ID, legacy.ID)
	}
	if domain.Type != legacy.Type {
		t.Errorf("Type: got %q, want %q", domain.Type, legacy.Type)
	}
	if domain.Content != legacy.Content {
		t.Errorf("Content: got %q, want %q", domain.Content, legacy.Content)
	}
	if !domain.UpdatedAt.Equal(ts) {
		t.Errorf("UpdatedAt: got %v, want %v", domain.UpdatedAt, ts)
	}
	if domain.GetString("title") != "Test" {
		t.Errorf("title: got %q, want %q", domain.GetString("title"), "Test")
	}

	// Round-trip back
	back := EntityFromDomain(domain)
	if back.ID != legacy.ID || back.Type != legacy.Type || back.Content != legacy.Content {
		t.Error("round-trip lost core fields")
	}
	if !back.ModTime.Equal(ts) {
		t.Errorf("ModTime: got %v, want %v", back.ModTime, ts)
	}
	if back.FilePath != "" {
		t.Errorf("FilePath should be empty after round-trip, got %q", back.FilePath)
	}
}

func TestEntityToDomain_PropertiesIsolated(t *testing.T) {
	legacy := &Entity{
		ID:         "A",
		Type:       "t",
		Properties: map[string]interface{}{"k": "v"},
	}
	domain := EntityToDomain(legacy)
	domain.Properties["k"] = "mutated"

	if legacy.Properties["k"] != "v" {
		t.Error("mutation leaked back to legacy entity")
	}
}

func TestEntityFromDomain_PropertiesIsolated(t *testing.T) {
	domain := &entity.Entity{
		ID:         "A",
		Type:       "t",
		Properties: map[string]interface{}{"k": "v"},
	}
	legacy := EntityFromDomain(domain)
	legacy.Properties["k"] = "mutated"

	if domain.Properties["k"] != "v" {
		t.Error("mutation leaked back to domain entity")
	}
}

func TestRelationRoundTrip(t *testing.T) {
	ts := time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)
	legacy := &Relation{
		From:       "A",
		Type:       "requires",
		To:         "B",
		Properties: map[string]interface{}{"weight": 5},
		Content:    "link note",
		FilePath:   "/rels/A--requires--B.md",
		ModTime:    ts,
	}

	domain := RelationToDomain(legacy)
	if domain.From != "A" || domain.Type != "requires" || domain.To != "B" {
		t.Error("key fields lost")
	}
	if domain.Content != "link note" {
		t.Errorf("Content: got %q", domain.Content)
	}
	if !domain.UpdatedAt.Equal(ts) {
		t.Errorf("UpdatedAt: got %v", domain.UpdatedAt)
	}

	back := RelationFromDomain(domain)
	if back.From != "A" || back.Type != "requires" || back.To != "B" {
		t.Error("round-trip lost key fields")
	}
	if back.FilePath != "" {
		t.Errorf("FilePath should be empty, got %q", back.FilePath)
	}
}

func TestRelationToDomain_NilProperties(t *testing.T) {
	legacy := &Relation{From: "A", Type: "r", To: "B"}
	domain := RelationToDomain(legacy)
	if domain.Properties != nil {
		t.Error("expected nil properties for nil input")
	}
}

func TestRelationFromDomain_NilProperties(t *testing.T) {
	domain := &entity.Relation{From: "A", Type: "r", To: "B"}
	legacy := RelationFromDomain(domain)
	if legacy.Properties != nil {
		t.Error("expected nil properties for nil input")
	}
}
