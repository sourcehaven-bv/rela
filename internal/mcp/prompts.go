// coverage-ignore: MCP prompt handlers - tested via integration tests
package mcp

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/Sourcehaven-BV/rela/internal/model"
)

func (s *Server) registerPrompts() {
	s.mcp.AddPrompt(promptAnalyzeTraceability(), s.handleAnalyzeTraceabilityPrompt)
	s.mcp.AddPrompt(promptReviewOrphans(), s.handleReviewOrphansPrompt)
	s.mcp.AddPrompt(promptSummarizeProject(), s.handleSummarizeProjectPrompt)
	s.mcp.AddPrompt(promptReviewEntity(), s.handleReviewEntityPrompt)
}

func promptAnalyzeTraceability() mcp.Prompt {
	return mcp.NewPrompt("analyze-traceability",
		mcp.WithPromptDescription("Analyze traceability coverage for an entity"),
		mcp.WithArgument("id",
			mcp.RequiredArgument(),
			mcp.ArgumentDescription("Entity ID to analyze (e.g. REQ-001)"),
		),
	)
}

func promptReviewOrphans() mcp.Prompt {
	return mcp.NewPrompt("review-orphans",
		mcp.WithPromptDescription("Review orphan entities and suggest connections"),
		mcp.WithArgument("type",
			mcp.ArgumentDescription("Filter by entity type (optional)"),
		),
	)
}

func promptSummarizeProject() mcp.Prompt {
	return mcp.NewPrompt("summarize-project",
		mcp.WithPromptDescription("Generate a project overview from the entity graph"),
	)
}

func promptReviewEntity() mcp.Prompt {
	return mcp.NewPrompt("review-entity",
		mcp.WithPromptDescription("Review an entity for completeness and quality"),
		mcp.WithArgument("id",
			mcp.RequiredArgument(),
			mcp.ArgumentDescription("Entity ID to review (e.g. REQ-001)"),
		),
	)
}

// --- Prompt Handlers ---

func (s *Server) handleAnalyzeTraceabilityPrompt(
	_ context.Context, request mcp.GetPromptRequest,
) (*mcp.GetPromptResult, error) {
	id := request.Params.Arguments["id"]
	if id == "" {
		return nil, fmt.Errorf("id argument is required")
	}

	entity, ok := s.graph.GetNode(id)
	if !ok {
		return nil, fmt.Errorf("entity not found: %s", id)
	}

	// Get entity details
	entityText, err := convertEntity(entity, s.graph, true)
	if err != nil {
		return nil, err
	}

	// Get trace trees
	traceFrom := s.graph.TraceFrom(id, 0)
	traceTo := s.graph.TraceTo(id, 0)

	var traceFromText, traceToText string
	if traceFrom != nil {
		traceFromText, _ = convertTraceResult(traceFrom)
	} else {
		traceFromText = "No downstream dependencies"
	}
	if traceTo != nil {
		traceToText, _ = convertTraceResult(traceTo)
	} else {
		traceToText = "No upstream dependencies"
	}

	// Build prompt
	content := fmt.Sprintf(`Analyze the traceability coverage for entity %s.

## Entity Details
%s

## Downstream Dependencies (trace from)
%s

## Upstream Dependencies (trace to)
%s

## Instructions
Please analyze:
1. Is this entity fully traceable? Are there missing upstream or downstream links?
2. Are there any broken chains in the traceability?
3. What entities should this be connected to but isn't?
4. Rate the overall traceability coverage (complete, partial, or poor).
5. Suggest specific relations that should be created to improve coverage.`,
		id, entityText, traceFromText, traceToText)

	return &mcp.GetPromptResult{
		Description: fmt.Sprintf("Traceability analysis for %s", id),
		Messages: []mcp.PromptMessage{
			mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent(content)),
		},
	}, nil
}

func (s *Server) handleReviewOrphansPrompt(
	_ context.Context, request mcp.GetPromptRequest,
) (*mcp.GetPromptResult, error) {
	entityType := request.Params.Arguments["type"]

	orphans := s.graph.FindOrphans()
	if entityType != "" {
		resolved := s.resolveType(entityType)
		var filtered []*model.Entity
		for _, o := range orphans {
			if o.Type == resolved {
				filtered = append(filtered, o)
			}
		}
		orphans = filtered
	}

	sortEntitiesByID(orphans)

	orphanText, err := convertEntitiesList(orphans)
	if err != nil {
		return nil, err
	}

	// Get available relation types
	relTypes := s.meta.RelationTypes()
	sort.Strings(relTypes)
	var relInfo strings.Builder
	for _, name := range relTypes {
		def, _ := s.meta.GetRelationDef(name)
		if def == nil {
			continue
		}
		fmt.Fprintf(&relInfo, "- %s: %s -> %s", name,
			strings.Join(def.From, ", "), strings.Join(def.To, ", "))
		if def.Description != "" {
			fmt.Fprintf(&relInfo, " (%s)", def.Description)
		}
		relInfo.WriteString("\n")
	}

	content := fmt.Sprintf(`Review the orphan entities in this project and suggest how to connect them.

## Orphan Entities (%d found)
%s

## Available Relation Types
%s

## Instructions
For each orphan entity:
1. Identify what relation(s) should be created to connect it to the graph
2. Suggest specific source and target entities for the relations
3. Explain why this connection makes sense
4. If an entity truly doesn't need connections, explain why it's acceptable as an orphan`,
		len(orphans), orphanText, relInfo.String())

	return &mcp.GetPromptResult{
		Description: "Review orphan entities",
		Messages: []mcp.PromptMessage{
			mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent(content)),
		},
	}, nil
}

func (s *Server) handleSummarizeProjectPrompt(
	_ context.Context, _ mcp.GetPromptRequest,
) (*mcp.GetPromptResult, error) {
	// Entity counts by type
	entityTypes := s.meta.EntityTypes()
	sort.Strings(entityTypes)
	var entityCounts strings.Builder
	totalEntities := 0
	for _, t := range entityTypes {
		count := len(s.graph.NodesByType(t))
		totalEntities += count
		def, _ := s.meta.GetEntityDef(t)
		label := t
		if def != nil {
			label = def.GetLabel()
		}
		fmt.Fprintf(&entityCounts, "- %s: %d\n", label, count)
	}

	// Relation counts by type
	relTypes := s.meta.RelationTypes()
	sort.Strings(relTypes)
	var relCounts strings.Builder
	totalRelations := 0
	for _, t := range relTypes {
		count := len(s.graph.RelationsOfType(t))
		totalRelations += count
		fmt.Fprintf(&relCounts, "- %s: %d\n", t, count)
	}

	// Analysis summary
	orphanCount := len(s.graph.FindOrphans())

	content := fmt.Sprintf(`Generate a comprehensive project summary based on the following data.

## Project Overview
- Total entities: %d
- Total relations: %d
- Orphan entities: %d

## Entity Counts by Type
%s

## Relation Counts by Type
%s

## Metamodel
- Version: %s
- Namespace: %s
- Entity types: %d
- Relation types: %d

## Instructions
Please provide:
1. A high-level summary of the project's architecture and structure
2. Assessment of the project's completeness and maturity
3. Key findings (e.g., entity types with most/fewest entries, potential gaps)
4. Recommendations for improving the project's traceability`,
		totalEntities, totalRelations, orphanCount,
		entityCounts.String(), relCounts.String(),
		s.meta.GetVersion(), s.meta.GetNamespace(),
		len(entityTypes), len(relTypes))

	return &mcp.GetPromptResult{
		Description: "Project summary",
		Messages: []mcp.PromptMessage{
			mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent(content)),
		},
	}, nil
}

func (s *Server) handleReviewEntityPrompt(
	_ context.Context, request mcp.GetPromptRequest,
) (*mcp.GetPromptResult, error) {
	id := request.Params.Arguments["id"]
	if id == "" {
		return nil, fmt.Errorf("id argument is required")
	}

	entity, ok := s.graph.GetNode(id)
	if !ok {
		return nil, fmt.Errorf("entity not found: %s", id)
	}

	entityText, err := convertEntity(entity, s.graph, true)
	if err != nil {
		return nil, err
	}

	// Get entity type schema
	def, _ := s.meta.GetEntityDef(entity.Type)
	var schemaText string
	if def != nil {
		schemaJSON, _ := marshalJSON(def.Properties)
		schemaText = schemaJSON
	} else {
		schemaText = "No schema available"
	}

	// Run validations for this entity
	errs := s.meta.ValidateEntity(entity)
	var validationText string
	if len(errs) == 0 {
		validationText = "All validations passed"
	} else {
		var msgs []string
		for _, e := range errs {
			msgs = append(msgs, "- "+e.Error())
		}
		validationText = strings.Join(msgs, "\n")
	}

	content := fmt.Sprintf(`Review entity %s for completeness and quality.

## Entity
%s

## Property Schema for type "%s"
%s

## Validation Results
%s

## Instructions
Please review this entity for:
1. Completeness: Are all required and recommended properties filled in?
2. Quality: Is the title descriptive? Is the content clear and well-structured?
3. Consistency: Do property values match the expected schema?
4. Suggestions: What improvements would you recommend?`,
		id, entityText, entity.Type, schemaText, validationText)

	return &mcp.GetPromptResult{
		Description: fmt.Sprintf("Entity review for %s", id),
		Messages: []mcp.PromptMessage{
			mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent(content)),
		},
	}, nil
}
