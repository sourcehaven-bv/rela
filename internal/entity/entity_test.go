package entity_test

import (
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	e := entity.New("FEAT-001", "feature")
	assert.Equal(t, "FEAT-001", e.ID)
	assert.Equal(t, "feature", e.Type)
	assert.NotNil(t, e.Properties)
	assert.Empty(t, e.Properties)
}

func TestGetSetString(t *testing.T) {
	e := entity.New("T-1", "ticket")
	e.SetString("status", "open")
	assert.Equal(t, "open", e.GetString("status"))
	assert.Equal(t, "", e.GetString("nonexistent"))
}

func TestConvenienceAccessors(t *testing.T) {
	e := entity.New("T-1", "ticket")
	e.SetString("title", "Fix bug")
	e.SetString("status", "open")
	e.SetString("description", "It's broken")

	assert.Equal(t, "Fix bug", e.Title())
	assert.Equal(t, "open", e.Status())
	assert.Equal(t, "It's broken", e.Description())
}

func TestGetAttribute(t *testing.T) {
	e := entity.New("T-1", "ticket")
	e.SetString("priority", "high")

	assert.Equal(t, "T-1", e.GetAttribute("id"))
	assert.Equal(t, "ticket", e.GetAttribute("type"))
	assert.Equal(t, "high", e.GetAttribute("priority"))
	assert.Nil(t, e.GetAttribute("missing"))
}

func TestGetAttributeString(t *testing.T) {
	e := entity.New("T-1", "ticket")
	e.Properties["count"] = 42

	assert.Equal(t, "T-1", e.GetAttributeString("id"))
	assert.Equal(t, "42", e.GetAttributeString("count"))
	assert.Equal(t, "", e.GetAttributeString("missing"))
}

func TestGetAttributeStrings(t *testing.T) {
	e := entity.New("T-1", "ticket")

	e.Properties["tags"] = []string{"bug", "urgent"}
	assert.Equal(t, []string{"bug", "urgent"}, e.GetAttributeStrings("tags"))

	e.Properties["mixed"] = []interface{}{"a", "b"}
	assert.Equal(t, []string{"a", "b"}, e.GetAttributeStrings("mixed"))

	assert.Nil(t, e.GetAttributeStrings("missing"))
	e.Properties["scalar"] = "not-a-list"
	assert.Nil(t, e.GetAttributeStrings("scalar"))
}

func TestClone(t *testing.T) {
	now := time.Now()
	e := entity.New("T-1", "ticket")
	e.SetString("title", "Original")
	e.Content = "body"
	e.UpdatedAt = now

	clone := e.Clone()

	require.Equal(t, e.ID, clone.ID)
	require.Equal(t, e.Type, clone.Type)
	require.Equal(t, e.Content, clone.Content)
	require.Equal(t, e.UpdatedAt, clone.UpdatedAt)
	require.Equal(t, e.Properties["title"], clone.Properties["title"])

	// Mutating clone does not affect original
	clone.SetString("title", "Changed")
	assert.Equal(t, "Original", e.GetString("title"))
}

func TestNewRelation(t *testing.T) {
	r := entity.NewRelation("FEAT-001", "requires", "REQ-001")
	assert.Equal(t, "FEAT-001", r.From)
	assert.Equal(t, "requires", r.Type)
	assert.Equal(t, "REQ-001", r.To)
}

func TestRelationKey(t *testing.T) {
	r := entity.NewRelation("A", "links", "B")
	assert.Equal(t, "A--links--B", r.Key())
}

func TestRelationClone(t *testing.T) {
	now := time.Now()
	r := entity.NewRelation("A", "links", "B")
	r.Properties = map[string]interface{}{"weight": 1}
	r.Content = "note"
	r.UpdatedAt = now

	clone := r.Clone()

	require.Equal(t, r.From, clone.From)
	require.Equal(t, r.Type, clone.Type)
	require.Equal(t, r.To, clone.To)
	require.Equal(t, r.Content, clone.Content)
	require.Equal(t, r.UpdatedAt, clone.UpdatedAt)
	require.Equal(t, r.Properties["weight"], clone.Properties["weight"])

	clone.Properties["weight"] = 99
	assert.Equal(t, 1, r.Properties["weight"])
}
