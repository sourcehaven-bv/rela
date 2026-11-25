package dataentry

import (
	"net/http"
)

// analyzeHandler owns GET /api/v1/_analyze. Extracted from App to take the
// endpoint off the god-object.
type analyzeHandler struct {
	// svc runs the read-only graph-analysis checks (TKT-N26KLB M5.1); it
	// holds its own {store, tracer, validator} and takes the metamodel
	// snapshot per call. wireValidation sets it, and setWorlds sets it
	// again once the schema's default world is known, both before serving.
	svc analyzeService
	// schema is the reloadable snapshot. Each request takes it once.
	schema func() *Schema
	// fullScriptDetail is the host-trust check for the rich script-error
	// envelope (App.allowFullScriptDetail). A closure because the security
	// layer is wired after NewApp.
	fullScriptDetail func(r *http.Request) bool
}

// newAnalyzeHandler wires the handler to app's live collaborators. The
// analysis service is set by wireValidation.
func newAnalyzeHandler(app *App) *analyzeHandler {
	return &analyzeHandler{
		schema:           app.State,
		fullScriptDetail: app.allowFullScriptDetail,
	}
}

// handleV1Analyze runs the analysis gated per principal, so no output filter
// is needed.
//
//archplan:auth=required
func (h *analyzeHandler) handleV1Analyze(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
		return
	}

	analysisResult := h.svc.runAnalysis(r.Context(), h.schema().Meta)

	visible := flattenIssues(analysisResult.Sections)

	result := APIAnalysisResult{
		Issues:  make([]APIIssue, 0, len(visible)),
		ByCheck: make(map[string]int),
	}

	for _, section := range analysisResult.Sections {
		if section.Truncated {
			result.TruncatedChecks = append(result.TruncatedChecks, section.Name)
		}
	}

	fullDetail := h.fullScriptDetail(r)

	for _, vi := range visible {
		issue, section := vi.issue, vi.section
		api := APIIssue{
			EntityID:    issue.EntityID,
			Face:        string(issue.Face),
			EntityType:  issue.EntityType,
			Title:       issue.Title,
			Message:     issue.Message,
			RuleMessage: issue.RuleMessage,
			Severity:    issue.Severity,
			CheckType:   section,
			Detail:      issue.Detail,
		}
		if issue.ScriptError != nil {
			env := buildScriptErrorEnvelope(issue.ScriptError, fullDetail, "")
			api.ScriptError = &env
		}
		result.Issues = append(result.Issues, api)
		result.ByCheck[section]++
		switch issue.Severity {
		case "error":
			result.Errors++
		case "warning":
			result.Warnings++
		}
	}

	writeV1JSON(w, http.StatusOK, result)
}

// visibleIssue pairs an analysis issue with its section name, carried
// through the ACL filter so the wire builder keeps the issue→check
// association without re-walking sections.
type visibleIssue struct {
	issue   AnalysisIssue
	section string
}

// flattenIssues walks the analysis sections into (issue, section) pairs for the
// wire builder. No filtering: runAnalysis already read gated, so the issues are
// the requester's visible slice — a hidden entity produced no issue and a
// visible entity was redacted before its value could reach a title or message
// (TKT-3FL2S6, replacing the former visibleAnalysisIssues output filter).
func flattenIssues(sections []AnalysisSection) []visibleIssue {
	var out []visibleIssue
	for _, section := range sections {
		for _, issue := range section.Issues {
			out = append(out, visibleIssue{issue: issue, section: section.Name})
		}
	}
	return out
}
