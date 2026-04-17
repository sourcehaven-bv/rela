package app_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/app"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

func TestNewBuildsStoreWithSchemas(t *testing.T) {
	fs := storage.NewMemFS()
	paths := &project.Context{
		Root:         "/proj",
		EntitiesDir:  "/proj/entities",
		RelationsDir: "/proj/relations",
		CacheDir:     "/proj/.rela",
	}
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"policy": {Plural: "policies", PropertyOrder: []string{"title", "status"}},
			"ticket": {}, // default plural = "tickets"
		},
	}

	a, err := app.New(app.Config{FS: fs, Paths: paths, Meta: meta})
	require.NoError(t, err)
	defer a.Close()

	require.NotNil(t, a.Store)
	require.NoError(t, a.Store.CreateEntity(context.Background(), &entity.Entity{
		ID:   "POL-1",
		Type: "policy",
	}))

	got, err := fs.ReadFile("/proj/entities/policies/POL-1.md")
	require.NoError(t, err)
	assert.Contains(t, string(got), "id: POL-1")
	assert.Contains(t, string(got), "type: policy")
}

func TestNewWithoutMeta(t *testing.T) {
	fs := storage.NewMemFS()
	paths := &project.Context{
		Root:         "/proj",
		EntitiesDir:  "/proj/entities",
		RelationsDir: "/proj/relations",
		CacheDir:     "/proj/.rela",
	}

	a, err := app.New(app.Config{FS: fs, Paths: paths})
	require.NoError(t, err)
	defer a.Close()

	require.NotNil(t, a.Store)
}
