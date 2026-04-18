// coverage-ignore: MCP prompt handlers - tested via integration tests
package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/Sourcehaven-BV/rela/internal/natsort"
	"github.com/Sourcehaven-BV/rela/internal/store"
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

	ctx := context.Background()
	st := s.ws.Store()
	e, getErr := st.GetEntity(ctx, id)
	if getErr != nil {
		return nil, fmt.Errorf("entity not found: %s", id)
	}

	entityText, err := convertStoreEntity(e, st, true)
	if err != nil {
		return nil, err
	}

	// Get trace trees
	tracer := s.ws.Tracer()
	traceFrom := tracer.TraceFrom(ctx, id, 0)
	traceTo := tracer.TraceTo(ctx, id, 0)

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

	ctx := context.Background()
	orphanIDs, _ := s.ws.Tracer().FindOrphans(ctx)

	st := s.ws.Store()
	var resolved string
	if entityType != "" {
		resolved = s.resolveType(entityType)
	}

	type orphanSummary struct {
		ID     string `json:"id"`
		Type   string `json:"type"`
		Title  string `json:"title,omitempty"`
		Status string `json:"status,omitempty"`
	}
	summaries := make([]orphanSummary, 0)
	for _, id := range orphanIDs {
		e, err := st.GetEntity(ctx, id)
		if err != nil {
			continue
		}
		if resolved != "" && e.Type != resolved {
			continue
		}
		summaries = append(summaries, orphanSummary{
			ID: e.ID, Type: e.Type, Title: e.Title(), Status: e.Status(),
		})
	}

	orphanText, err := marshalJSON(summaries)
	if err != nil {
		return nil, err
	}

	// Get available relation types
	meta := s.ws.Meta()
	relTypes := meta.RelationTypes()
	natsort.Strings(relTypes)
	var relInfo strings.Builder
	for _, name := range relTypes {
		def, _ := meta.GetRelationDef(name)
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
		len(summaries), orphanText, relInfo.String())

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
	ctx := context.Background()
	meta := s.ws.Meta()
	st := s.ws.Store()
	entityTypes := meta.EntityTypes()
	natsort.Strings(entityTypes)
	var entityCounts strings.Builder
	totalEntities := 0
	for _, t := range entityTypes {
		count, _ := st.CountEntities(ctx, store.EntityQuery{Type: t})
		totalEntities += count
		def, _ := meta.GetEntityDef(t)
		label := t
		if def != nil {
			label = def.GetLabel()
		}
		fmt.Fprintf(&entityCounts, "- %s: %d\n", label, count)
	}

	// Relation counts by type
	relTypes := meta.RelationTypes()
	natsort.Strings(relTypes)
	var relCounts strings.Builder
	totalRelations := 0
	for _, t := range relTypes {
		count, _ := st.CountRelations(ctx, store.RelationQuery{Type: t})
		totalRelations += count
		fmt.Fprintf(&relCounts, "- %s: %d\n", t, count)
	}

	// Analysis summary
	orphanIDs, _ := s.ws.Tracer().FindOrphans(ctx)
	orphanCount := len(orphanIDs)

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
		meta.GetVersion(), meta.GetNamespace(),
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

	st := s.ws.Store()
	entity, getErr := st.GetEntity(context.Background(), id)
	if getErr != nil {
		return nil, fmt.Errorf("entity not found: %s", id)
	}

	entityText, err := convertStoreEntity(entity, st, true)
	if err != nil {
		return nil, err
	}

	// Get entity type schema
	meta := s.ws.Meta()
	def, _ := meta.GetEntityDef(entity.Type)
	var schemaText string
	if def != nil {
		schemaJSON, _ := marshalJSON(def.Properties)
		schemaText = schemaJSON
	} else {
		schemaText = "No schema available"
	}

	// Run validations for this entity
	errs := meta.ValidateEntity(entity.ID, entity.Type, entity.Properties)
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
