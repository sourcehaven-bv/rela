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

func TestFSFactoryOpensWorkingStore(t *testing.T) {
	fs := storage.NewMemFS()
	paths := &project.Context{
		Root:         "/proj",
		EntitiesDir:  "/proj/entities",
		RelationsDir: "/proj/relations",
		CacheDir:     "/proj/.rela",
	}
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"policy": {Plural: "policies"},
		},
	}

	factory := &app.FSFactory{FS: fs, Paths: paths}
	s, err := factory.OpenStore(meta)
	require.NoError(t, err)
	defer s.Close()

	require.NoError(t, s.CreateEntity(context.Background(), &entity.Entity{
		ID:   "POL-1",
		Type: "policy",
	}))

	data, err := fs.ReadFile("/proj/entities/policies/POL-1.md")
	require.NoError(t, err)
	assert.Contains(t, string(data), "id: POL-1")
}

func TestFSFactoryOpenStoreReturnsIndependentStores(t *testing.T) {
	fs := storage.NewMemFS()
	paths := &project.Context{
		Root:         "/proj",
		EntitiesDir:  "/proj/entities",
		RelationsDir: "/proj/relations",
		CacheDir:     "/proj/.rela",
	}

	factory := &app.FSFactory{FS: fs, Paths: paths}
	s1, err := factory.OpenStore(nil)
	require.NoError(t, err)
	defer s1.Close()

	s2, err := factory.OpenStore(nil)
	require.NoError(t, err)
	defer s2.Close()

	assert.NotSame(t, s1, s2, "each OpenStore call returns a fresh store")
}
