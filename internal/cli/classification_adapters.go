package cli

import (
	"fmt"
	"io/fs"
	"slices"

	"github.com/Sourcehaven-BV/rela/internal/classification"
	"github.com/Sourcehaven-BV/rela/internal/datamigration"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// classificationShape describes the schema to the classification package,
// which imports no rela package.
func classificationShape(mm *metamodel.Metamodel) classification.Shape {
	shape := classification.Shape{
		Entities:  make(map[string]classification.TypeShape, len(mm.Entities)),
		Relations: make(map[string]classification.RelationShape, len(mm.Relations)),
	}
	for name, def := range mm.Entities {
		fields := propertyOrder(def.GetPropertyOrder(), def.Properties)
		if !slices.Contains(fields, classification.BodyField) {
			fields = append(fields, classification.BodyField)
		}
		shape.Entities[name] = classification.TypeShape{
			Fields:        fields,
			DisplayFields: def.DisplayProperties(),
			OpaqueID:      !def.IsManualID(),
		}
	}
	for name, def := range mm.Relations {
		fields := sortedKeys(def.Properties)
		if def.Content && !slices.Contains(fields, classification.BodyField) {
			fields = append(fields, classification.BodyField)
		}
		ends := make([]classification.Ends, 0, len(def.From)*len(def.To))
		for _, from := range def.From {
			for _, to := range def.To {
				ends = append(ends, classification.Ends{From: mm.ResolveAlias(from), To: mm.ResolveAlias(to)})
			}
		}
		shape.Relations[name] = classification.RelationShape{Fields: fields, Ends: ends}
	}
	return shape
}

// propertyOrder returns the declared order, then any property the order
// misses (a programmatically built metamodel has no order) sorted by name.
func propertyOrder(order []string, props map[string]metamodel.PropertyDef) []string {
	fields := make([]string, 0, len(props))
	seen := make(map[string]bool, len(props))
	for _, name := range order {
		if _, ok := props[name]; ok && !seen[name] {
			fields = append(fields, name)
			seen[name] = true
		}
	}
	for _, name := range sortedKeys(props) {
		if !seen[name] {
			fields = append(fields, name)
		}
	}
	return fields
}

// classificationRenames returns the renames recorded in the project's
// data-migration files, in history order.
func classificationRenames(project fs.FS) ([]classification.Rename, error) {
	files, err := datamigration.LoadDir(project)
	if err != nil {
		return nil, fmt.Errorf("read migrations for renames: %w", err)
	}
	var out []classification.Rename
	for _, f := range files {
		for _, r := range f.Renames() {
			var kind classification.RenameKind
			switch r.Kind {
			case datamigration.RenameProperty:
				kind = classification.RenameProperty
			case datamigration.RenameEntityType:
				kind = classification.RenameEntityType
			case datamigration.RenameRelationType:
				kind = classification.RenameRelationType
			}
			out = append(out, classification.Rename{Kind: kind, Owner: r.Owner, From: r.From, To: r.To})
		}
	}
	return out, nil
}
