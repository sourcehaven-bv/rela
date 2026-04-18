package cli

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/filter"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

var (
	listWhere []string
	listSort  string
	listDesc  bool
)

var listCmd = &cobra.Command{
	Use:   "list [type]",
	Short: "List entities",
	Long: `Lists all entities, optionally filtered by type and properties.

Filter Syntax:
  --where "property=value"      Exact match (supports glob patterns with *)
  --where "property!=value"     Not equal
  --where "property<value"      Less than (date/integer)
  --where "property<=value"     Less than or equal (date/integer)
  --where "property>value"      Greater than (date/integer)
  --where "property>=value"     Greater than or equal (date/integer)
  --where "property=~pattern"   Regex match (string)

Examples:
  rela list                                    # List all entities
  rela list requirements                       # List all requirements
  rela list req                                # Alias works too
  rela list control --where "status=accepted"  # Filter by status
  rela list control --where "iso27001=A.9.*"   # Glob pattern match
  rela list evidence --where "valid_until<2025-02-01"  # Date comparison
  rela list risk --where "risk_score>=5"       # Integer comparison
  rela list control --where "status=implemented" --where "applicability=applicable"  # Multiple filters (AND)
  rela list control --sort iso27001            # Sort by property
  rela list evidence --sort valid_until --desc # Sort descending`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		st := ws.Store()

		q := store.EntityQuery{}
		var entityTypeName string

		if len(args) > 0 {
			resolvedType, _, err := resolveEntityType(args[0])
			if err != nil {
				return err
			}
			entityTypeName = resolvedType
			q.Type = resolvedType
		}

		var entities []*entity.Entity
		for e, err := range st.ListEntities(ctx, q) {
			if err != nil {
				return err
			}
			entities = append(entities, e)
		}

		// Parse and apply filters
		if len(listWhere) > 0 {
			if entityTypeName == "" {
				return errors.New("--where filters require specifying an entity type")
			}

			entityDef, ok := meta.GetEntityDef(entityTypeName)
			if !ok {
				return fmt.Errorf("unknown entity type: %s", entityTypeName)
			}

			filters, err := filter.ParseAll(listWhere)
			if err != nil {
				return fmt.Errorf("invalid filter: %w", err)
			}

			for _, f := range filters {
				if _, ok := entityDef.Properties[f.Property]; !ok {
					return fmt.Errorf("unknown property %q for entity type %q", f.Property, entityTypeName)
				}
			}

			var filtered []*entity.Entity
			for _, e := range entities {
				matches, err := filter.MatchAll(storeEntityRecord(e), filters, entityDef, meta)
				if err != nil {
					return fmt.Errorf("filter error: %w", err)
				}
				if matches {
					filtered = append(filtered, e)
				}
			}
			entities = filtered
		}

		// Apply sorting
		if listSort != "" {
			if entityTypeName == "" {
				return errors.New("--sort requires specifying an entity type")
			}

			entityDef, ok := meta.GetEntityDef(entityTypeName)
			if !ok {
				return fmt.Errorf("unknown entity type: %s", entityTypeName)
			}

			if listSort == "id" {
				filter.SortByID(entities, storeEntityRecord, listDesc)
			} else {
				propDef, ok := entityDef.Properties[listSort]
				if !ok {
					return fmt.Errorf("unknown property %q for entity type %q", listSort, entityTypeName)
				}
				filter.Sort(entities, storeEntityRecord, listSort, &propDef, meta, listDesc)
			}
		} else {
			filter.SortByID(entities, storeEntityRecord, listDesc)
		}

		if len(entities) == 0 {
			out.WriteMessage("No entities found")
			return nil
		}

		return out.WriteEntitiesWithSummary(entities)
	},
}

func init() {
	listCmd.Flags().StringArrayVar(&listWhere, "where", nil, `Filter by property (e.g., --where "status=draft")`)
	listCmd.Flags().StringVar(&listSort, "sort", "", "Sort by property (or 'id')")
	listCmd.Flags().BoolVar(&listDesc, "desc", false, "Sort descending")

	rootCmd.AddCommand(listCmd)
}
