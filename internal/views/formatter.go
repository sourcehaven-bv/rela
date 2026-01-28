package views

import (
	"encoding/json"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/graph"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
)

// Format formats a view result as JSON or YAML
func Format(result *ViewResult, format string, g *graph.Graph, meta *metamodel.Metamodel) (string, error) {
	output := buildOutput(result, g, meta)

	switch format {
	case "json":
		data, err := json.MarshalIndent(output, "", "  ")
		if err != nil {
			return "", fmt.Errorf("failed to marshal JSON: %w", err)
		}
		return string(data), nil

	case "yaml":
		data, err := yaml.Marshal(output)
		if err != nil {
			return "", fmt.Errorf("failed to marshal YAML: %w", err)
		}
		return string(data), nil

	default:
		return "", fmt.Errorf("unknown format: %s", format)
	}
}

func buildOutput(result *ViewResult, g *graph.Graph, meta *metamodel.Metamodel) map[string]interface{} {
	output := make(map[string]interface{})

	// Add entry entity if present
	if result.Entry != nil {
		output["entry"] = formatEntity(result.Entry, result.OutputConfig, g, meta)
	}

	// Add collections
	collections := make(map[string]interface{})
	for name, entities := range result.Collections {
		// Check if this is a grouped collection
		if groupInfo, ok := result.GroupedCollections[name]; ok {
			collections[name] = formatGroupedCollection(entities, groupInfo, result.OutputConfig, g, meta)
		} else {
			formatted := make([]interface{}, 0, len(entities))
			for _, entity := range entities {
				formatted = append(formatted, formatEntity(entity, result.OutputConfig, g, meta))
			}
			collections[name] = formatted
		}
	}
	output["collections"] = collections

	// Add exported relations
	if len(result.Relations) > 0 {
		relations := make(map[string]interface{})
		for name, rels := range result.Relations {
			formatted := make([]interface{}, 0, len(rels))
			for _, rel := range rels {
				formatted = append(formatted, formatRelation(rel, result.OutputConfig))
			}
			relations[name] = formatted
		}
		output["relations"] = relations
	}

	return output
}

//nolint:gocognit,nestif // Entity formatting is inherently complex
func formatEntity(
	entity *model.Entity, config OutputDef, g *graph.Graph, _ *metamodel.Metamodel,
) map[string]interface{} {
	result := map[string]interface{}{
		"id":   entity.ID,
		"type": entity.Type,
	}

	if len(entity.Properties) > 0 {
		result["properties"] = entity.Properties
	}

	// Include content if requested
	if config.IncludeContent && entity.Content != "" {
		result["content"] = entity.Content
	}

	// Include relations
	outgoing := g.OutgoingEdges(entity.ID)
	incoming := g.IncomingEdges(entity.ID)

	if len(outgoing) > 0 || len(incoming) > 0 {
		relations := make(map[string]interface{})

		if len(outgoing) > 0 {
			outgoingMap := make(map[string][]interface{})
			for _, edge := range outgoing {
				if config.ResolveRelationTitles {
					// Resolve target entity title
					if target, ok := g.GetNode(edge.To); ok {
						outgoingMap[edge.Type] = append(outgoingMap[edge.Type], map[string]interface{}{
							"id":    edge.To,
							"title": target.Title(),
						})
					} else {
						outgoingMap[edge.Type] = append(outgoingMap[edge.Type], map[string]interface{}{
							"id": edge.To,
						})
					}
				} else {
					outgoingMap[edge.Type] = append(outgoingMap[edge.Type], edge.To)
				}
			}
			relations["outgoing"] = outgoingMap
		}

		if len(incoming) > 0 {
			incomingMap := make(map[string][]interface{})
			for _, edge := range incoming {
				if config.ResolveRelationTitles {
					// Resolve source entity title
					if source, ok := g.GetNode(edge.From); ok {
						incomingMap[edge.Type] = append(incomingMap[edge.Type], map[string]interface{}{
							"id":    edge.From,
							"title": source.Title(),
						})
					} else {
						incomingMap[edge.Type] = append(incomingMap[edge.Type], map[string]interface{}{
							"id": edge.From,
						})
					}
				} else {
					incomingMap[edge.Type] = append(incomingMap[edge.Type], edge.From)
				}
			}
			relations["incoming"] = incomingMap
		}

		result["relations"] = relations
	}

	return result
}

func formatGroupedCollection(
	entities []*model.Entity,
	groupInfo GroupingInfo,
	config OutputDef,
	g *graph.Graph,
	meta *metamodel.Metamodel,
) map[string]interface{} {
	// Extract property path from group_by (e.g., "properties.domain")
	groupByPath := strings.TrimPrefix(groupInfo.GroupBy, "properties.")

	grouped := make(map[string][]interface{})

	for _, entity := range entities {
		// Get the grouping key
		var key string
		if strings.HasPrefix(groupInfo.GroupBy, "properties.") {
			if val, ok := entity.Properties[groupByPath]; ok {
				key = fmt.Sprintf("%v", val)
			} else {
				key = "_ungrouped"
			}
		} else {
			// Direct property access (e.g., "type")
			switch groupInfo.GroupBy {
			case "type":
				key = entity.Type
			case "id":
				key = entity.ID
			default:
				key = "_ungrouped"
			}
		}

		if grouped[key] == nil {
			grouped[key] = make([]interface{}, 0)
		}

		grouped[key] = append(grouped[key], formatEntity(entity, config, g, meta))
	}

	result := make(map[string]interface{})
	for k, v := range grouped {
		result[k] = v
	}
	return result
}

func formatRelation(rel ExportedRelation, config OutputDef) map[string]interface{} {
	result := map[string]interface{}{
		"from": rel.From,
		"to":   rel.To,
		"type": rel.Type,
	}

	// Include content if requested
	if config.IncludeContent && rel.Content != "" {
		result["content"] = rel.Content
	}

	return result
}
