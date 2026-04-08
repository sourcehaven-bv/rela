package dataentry

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/Sourcehaven-BV/rela/internal/natsort"
)

// graphNode is a JSON-serializable node for the graph visualization.
type graphNode struct {
	ID         string            `json:"id"`
	Type       string            `json:"type"`
	Title      string            `json:"title"`
	Properties map[string]string `json:"properties,omitempty"`
}

// graphEdge is a JSON-serializable edge for the graph visualization.
type graphEdge struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Type   string `json:"type"`
}

// graphEntityType describes an entity type for the filter sidebar.
type graphEntityType struct {
	Type  string `json:"type"`
	Label string `json:"label"`
	Color string `json:"color"`
	Count int    `json:"count"`
}

// graphRelationType describes a relation type for the filter sidebar.
type graphRelationType struct {
	Type  string `json:"type"`
	Label string `json:"label"`
	Count int    `json:"count"`
}

// graphMetaProperty describes a property on an entity type for the detail panel.
type graphMetaProperty struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// graphMetaEntity describes entity type metadata.
type graphMetaEntity struct {
	Type       string              `json:"type"`
	Label      string              `json:"label"`
	Properties []graphMetaProperty `json:"properties"`
}

// graphMetaRelation describes relation type metadata.
type graphMetaRelation struct {
	Type string   `json:"type"`
	From []string `json:"from"`
	To   []string `json:"to"`
}

// graphDataResponse is the top-level response for /api/graph-data.
type graphDataResponse struct {
	Nodes         []graphNode         `json:"nodes"`
	Edges         []graphEdge         `json:"edges"`
	EntityTypes   []graphEntityType   `json:"entityTypes"`
	RelationTypes []graphRelationType `json:"relationTypes"`
	Meta          graphMetaData       `json:"meta"`
}

type graphMetaData struct {
	Entities  []graphMetaEntity   `json:"entities"`
	Relations []graphMetaRelation `json:"relations"`
}

// entityTypeColors maps entity types to display colors.
// Uses a curated palette that works well on both light and glass backgrounds.
var entityTypeColors = []string{
	"#6366f1", // indigo
	"#8b5cf6", // violet
	"#06b6d4", // cyan
	"#10b981", // emerald
	"#f59e0b", // amber
	"#ef4444", // red
	"#ec4899", // pink
	"#14b8a6", // teal
	"#f97316", // orange
	"#84cc16", // lime
	"#a855f7", // purple
	"#0ea5e9", // sky
	"#64748b", // slate
	"#d946ef", // fuchsia
	"#22d3ee", // cyan bright
	"#facc15", // yellow
}

// handleGraphData returns all graph data as JSON for the visualization.
func (a *App) handleGraphData(w http.ResponseWriter, r *http.Request) {
	mode := r.URL.Query().Get("mode")
	if mode == "" {
		mode = "content"
	}

	var resp graphDataResponse

	if mode == "metamodel" {
		resp = a.buildMetamodelGraphData()
	} else {
		resp = a.buildContentGraphData()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp) //nolint:errcheck // best-effort JSON response
}

func (a *App) buildContentGraphData() graphDataResponse {
	allNodes := a.Graph().AllNodes()
	allEdges := a.Graph().AllEdges()

	// Collect entity types
	entityTypes := a.Meta().EntityTypes()
	natsort.Strings(entityTypes)

	etList := make([]graphEntityType, 0, len(entityTypes))
	for i, et := range entityTypes {
		// Use metamodel color if available, otherwise cycle through palette
		color := entityTypeColors[i%len(entityTypeColors)]
		if entDef, ok := a.Meta().GetEntityDef(et); ok && entDef.Color != "" {
			color = entDef.Color
		}

		label := et
		if entDef, ok := a.Meta().GetEntityDef(et); ok && entDef.Label != "" {
			label = entDef.Label
		}
		count := len(a.Graph().NodesByType(et))
		etList = append(etList, graphEntityType{Type: et, Label: label, Color: color, Count: count})
	}

	// Build nodes
	nodes := make([]graphNode, 0, len(allNodes))
	for _, e := range allNodes {
		props := make(map[string]string)
		for k, v := range e.Properties {
			if v != nil {
				props[k] = fmt.Sprintf("%v", v)
			}
		}
		nodes = append(nodes, graphNode{
			ID:         e.ID,
			Type:       e.Type,
			Title:      a.entityDisplayTitle(e),
			Properties: props,
		})
	}

	// Build edges
	edges := make([]graphEdge, 0, len(allEdges))
	relTypeCounts := make(map[string]int)
	for _, rel := range allEdges {
		edges = append(edges, graphEdge{
			Source: rel.From,
			Target: rel.To,
			Type:   rel.Type,
		})
		relTypeCounts[rel.Type]++
	}

	// Collect relation types
	relTypes := a.Meta().RelationTypes()
	natsort.Strings(relTypes)
	rtList := make([]graphRelationType, 0, len(relTypes))
	for _, rt := range relTypes {
		label := rt
		if relDef, ok := a.Meta().GetRelationDef(rt); ok && relDef.Label != "" {
			label = relDef.Label
		}
		rtList = append(rtList, graphRelationType{Type: rt, Label: label, Count: relTypeCounts[rt]})
	}

	// Build meta info
	metaData := a.buildMetaInfo(entityTypes, relTypes)

	return graphDataResponse{
		Nodes:         nodes,
		Edges:         edges,
		EntityTypes:   etList,
		RelationTypes: rtList,
		Meta:          metaData,
	}
}

func (a *App) buildMetamodelGraphData() graphDataResponse {
	entityTypes := a.Meta().EntityTypes()
	natsort.Strings(entityTypes)

	etList := make([]graphEntityType, 0, len(entityTypes))
	nodes := make([]graphNode, 0, len(entityTypes))

	for i, et := range entityTypes {
		color := entityTypeColors[i%len(entityTypeColors)]
		if entDef, ok := a.Meta().GetEntityDef(et); ok && entDef.Color != "" {
			color = entDef.Color
		}

		label := et
		if entDef, ok := a.Meta().GetEntityDef(et); ok && entDef.Label != "" {
			label = entDef.Label
		}

		// Build properties as metadata for each entity type node
		props := make(map[string]string)
		if entDef, ok := a.Meta().GetEntityDef(et); ok {
			propNames := make([]string, 0, len(entDef.Properties))
			for pn := range entDef.Properties {
				propNames = append(propNames, pn)
			}
			natsort.Strings(propNames)
			for _, pn := range propNames {
				pd := entDef.Properties[pn]
				props[pn] = pd.Type
			}
		}

		count := len(a.Graph().NodesByType(et))
		etList = append(etList, graphEntityType{Type: et, Label: label, Color: color, Count: count})
		nodes = append(nodes, graphNode{
			ID:         et,
			Type:       et,
			Title:      label,
			Properties: props,
		})
	}

	// Build edges from relation definitions (from-type → to-type)
	relTypes := a.Meta().RelationTypes()
	natsort.Strings(relTypes)

	var edges []graphEdge
	relTypeCounts := make(map[string]int)

	for _, rt := range relTypes {
		relDef, ok := a.Meta().GetRelationDef(rt)
		if !ok {
			continue
		}
		for _, fromType := range relDef.From {
			for _, toType := range relDef.To {
				edges = append(edges, graphEdge{
					Source: fromType,
					Target: toType,
					Type:   rt,
				})
				relTypeCounts[rt]++
			}
		}
	}

	rtList := make([]graphRelationType, 0, len(relTypes))
	for _, rt := range relTypes {
		label := rt
		if relDef, ok := a.Meta().GetRelationDef(rt); ok && relDef.Label != "" {
			label = relDef.Label
		}
		rtList = append(rtList, graphRelationType{Type: rt, Label: label, Count: relTypeCounts[rt]})
	}

	metaData := a.buildMetaInfo(entityTypes, relTypes)

	return graphDataResponse{
		Nodes:         nodes,
		Edges:         edges,
		EntityTypes:   etList,
		RelationTypes: rtList,
		Meta:          metaData,
	}
}

func (a *App) buildMetaInfo(entityTypes, relTypes []string) graphMetaData {
	metaEntities := make([]graphMetaEntity, 0, len(entityTypes))
	for _, et := range entityTypes {
		me := graphMetaEntity{Type: et, Label: et}
		if entDef, ok := a.Meta().GetEntityDef(et); ok {
			if entDef.Label != "" {
				me.Label = entDef.Label
			}
			propNames := make([]string, 0, len(entDef.Properties))
			for pn := range entDef.Properties {
				propNames = append(propNames, pn)
			}
			natsort.Strings(propNames)
			for _, pn := range propNames {
				pd := entDef.Properties[pn]
				me.Properties = append(me.Properties, graphMetaProperty{Name: pn, Type: pd.Type})
			}
		}
		metaEntities = append(metaEntities, me)
	}

	metaRelations := make([]graphMetaRelation, 0, len(relTypes))
	for _, rt := range relTypes {
		mr := graphMetaRelation{Type: rt}
		if relDef, ok := a.Meta().GetRelationDef(rt); ok {
			mr.From = relDef.From
			mr.To = relDef.To
		}
		metaRelations = append(metaRelations, mr)
	}

	return graphMetaData{
		Entities:  metaEntities,
		Relations: metaRelations,
	}
}
