package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Sourcehaven-BV/rela/internal/filter"
	"github.com/Sourcehaven-BV/rela/internal/model"
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
		var entities []*model.Entity
		var entityTypeName string

		if len(args) > 0 {
			// Filter by type (resolveEntityType handles aliases and plural forms)
			typeName := args[0]

			resolvedType, _, err := resolveEntityType(typeName)
			if err != nil {
				return err
			}

			entityTypeName = resolvedType
			entities = ws.EntitiesByType(resolvedType)
		} else {
			// All entities
			entities = ws.AllEntities()
		}

		// Parse and apply filters
		if len(listWhere) > 0 {
			// Filters require a specific entity type
			if entityTypeName == "" {
				return fmt.Errorf("--where filters require specifying an entity type")
			}

			entityDef, ok := meta.GetEntityDef(entityTypeName)
			if !ok {
				return fmt.Errorf("unknown entity type: %s", entityTypeName)
			}

			// Parse all filter expressions
			filters, err := filter.ParseAll(listWhere)
			if err != nil {
				return fmt.Errorf("invalid filter: %w", err)
			}

			// Validate all filters reference valid properties
			for _, f := range filters {
				if _, ok := entityDef.Properties[f.Property]; !ok {
					return fmt.Errorf("unknown property %q for entity type %q", f.Property, entityTypeName)
				}
			}

			// Apply filters
			filtered := make([]*model.Entity, 0)
			for _, e := range entities {
				matches, err := filter.MatchAll(e, filters, entityDef, meta)
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
			// Sort requires a specific entity type
			if entityTypeName == "" {
				return fmt.Errorf("--sort requires specifying an entity type")
			}

			entityDef, ok := meta.GetEntityDef(entityTypeName)
			if !ok {
				return fmt.Errorf("unknown entity type: %s", entityTypeName)
			}

			// Special case: sort by "id"
			if listSort == "id" {
				filter.SortByID(entities, listDesc)
			} else {
				propDef, ok := entityDef.Properties[listSort]
				if !ok {
					return fmt.Errorf("unknown property %q for entity type %q", listSort, entityTypeName)
				}

				filter.Sort(entities, listSort, &propDef, meta, listDesc)
			}
		} else {
			// Default sort by ID
			filter.SortByID(entities, listDesc)
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
