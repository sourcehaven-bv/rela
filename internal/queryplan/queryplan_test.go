package queryplan

import (
	"reflect"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

func testMeta() *metamodel.Metamodel {
	return &metamodel.Metamodel{Entities: map[string]metamodel.EntityDef{
		"task": {Properties: map[string]metamodel.PropertyDef{
			"status": {Type: metamodel.PropertyTypeString},
			"owner":  {Type: metamodel.PropertyTypeString},
			"tags":   {Type: metamodel.PropertyTypeString, List: true},
			"count":  {Type: metamodel.PropertyTypeInteger},
		}},
	}}
}

func TestStaticIndexSpecsCollectsAndCanonicalizesStaticQueries(t *testing.T) {
	t.Parallel()
	cfg := &dataentryconfig.Config{
		Dashboard: &dataentryconfig.DashboardConfig{Cards: []dataentryconfig.DashboardCard{
			{Query: "type:task prop:status=open prop:owner=alice"},
			{Query: "type:task prop:owner=bob prop:status=done"},
		}},
		NextActions: map[string]dataentryconfig.NextActionSource{
			"stale": {
				Query: "type:task prop:status=stale",
				Actions: []dataentryconfig.NextActionOffer{{PickOne: &dataentryconfig.NextActionPickOne{
					Query: "type:task prop:owner=alice", Action: "open",
				}}},
			},
		},
	}
	want := []store.DerivedObjectSpec{
		{Kind: store.DerivedQueryIndex, Type: "task", Properties: []string{"owner"}},
		{Kind: store.DerivedQueryIndex, Type: "task", Properties: []string{"owner", "status"}},
		{Kind: store.DerivedQueryIndex, Type: "task", Properties: []string{"status"}},
	}
	if got := StaticIndexSpecs(cfg, testMeta()); !reflect.DeepEqual(got, want) {
		t.Fatalf("StaticIndexSpecs() = %#v, want %#v", got, want)
	}
}

func TestStaticIndexSpecsSkipsUnsupportedShapes(t *testing.T) {
	t.Parallel()
	queries := []string{
		"prop:status=open",                 // no type
		"type:task,other prop:status=open", // multiple types
		"type:task prop:status=",           // empty semantics
		"type:task prop:status!=open",      // not equal
		"type:task prop:status=op*",        // glob
		"type:task prop:tags=x",            // list
		"type:task prop:count=3",           // typed
		"type:task prop:missing=x",         // undeclared
		"type:task prop:status=open words", // free text
	}
	cfg := &dataentryconfig.Config{Dashboard: &dataentryconfig.DashboardConfig{}}
	for _, query := range queries {
		cfg.Dashboard.Cards = append(cfg.Dashboard.Cards, dataentryconfig.DashboardCard{Query: query})
	}
	if got := StaticIndexSpecs(cfg, testMeta()); len(got) != 0 {
		t.Fatalf("StaticIndexSpecs() = %#v, want none", got)
	}
}

func TestLoadStaticIndexSpecsRejectsIncompleteConfig(t *testing.T) {
	t.Parallel()
	if got, err := LoadStaticIndexSpecs([]byte("dashboard: ["), testMeta()); err == nil || got != nil {
		t.Fatalf("LoadStaticIndexSpecs() = %#v, %v; want nil specs and an error", got, err)
	}
}
