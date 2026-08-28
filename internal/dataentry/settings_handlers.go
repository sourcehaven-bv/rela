package dataentry

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
)

// --- JSON API types ---
// JSON shapes for the data-entry API (analysis results, settings, palette).
// The legacy /api/ entity/relation CRUD surface was removed (TKT-N26KLB);
// the live v1 API lives in api_v1.go.

// APIAnalysisResult is the JSON representation of analysis results.
type APIAnalysisResult struct {
	Errors   int            `json:"errors"`
	Warnings int            `json:"warnings"`
	Issues   []APIIssue     `json:"issues"`
	ByCheck  map[string]int `json:"byCheck"`

	// TruncatedChecks names the checks that found more issues than they
	// returned, so the UI can mark those lists as incomplete (TKT-1ESTYJ).
	//
	// Per-CHECK rather than one global flag: "duplicates is truncated" is
	// actionable where "something was truncated" is not, and the response
	// is a flat issue list, so the section-level flag would otherwise be
	// lost. Empty (omitted) when every check reported in full.
	//
	// Counts in ByCheck are counts of RETURNED issues; for a truncated
	// check that is the cap, not the true total, which is deliberately
	// not computed.
	TruncatedChecks []string `json:"truncatedChecks,omitempty"`
}

// APIIssue is the JSON representation of a single analysis issue.
type APIIssue struct {
	EntityID   string `json:"entityId"`
	EntityType string `json:"entityType"`
	Title      string `json:"title,omitempty"`
	Message    string `json:"message"`
	Severity   string `json:"severity"` // "error" or "warning"
	CheckType  string `json:"checkType"`

	// Detail carries optional structured specifics about why the issue
	// fired, beyond the flat Message. For content required-headers
	// violations it holds the missing exact headers. Absent (omitempty)
	// on rows with no structured detail; the frontend reveals it in an
	// expandable detail row under the message.
	Detail []string `json:"detail,omitempty"`

	// ScriptError carries the structured Lua-failure envelope for
	// validation script-error rows. Absent (omitempty) on every
	// other row. The frontend uses presence as the discriminator:
	// rows with scriptError open the ScriptErrorDialog instead of
	// navigating to an entity. Same loopback gating as the
	// action-surface envelope (security.AllowFullScriptDetail).
	ScriptError *ScriptErrorEnvelope `json:"scriptError,omitempty"`
}

// writeJSON writes a JSON response with 200 OK status.
func writeJSON(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(data)
}

// writeJSONError writes a JSON error response.
func writeJSONError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}

// writeForbiddenIfACLDenied checks whether err is an ACL deny and, if
// so, emits the structured 403 body documented in TKT-GN5LN. Returns
// true when the response has been written and the caller should
// return. The structured body — `{error, rule_kind, rule_id, reason}`
// — lets the SPA surface the specific rule that fired (the AWS IAM
// lesson: opaque denials are unsupportable).
//
// Every handler that calls a write entry point on
// [entitymanager.EntityManager] must invoke this *before* falling
// back to the generic 500 path. The check is cheap (an errors.As
// type assertion) and centralizing the 403 body shape here keeps the
// wire contract identical across all handlers.
func writeForbiddenIfACLDenied(w http.ResponseWriter, err error) bool {
	var fe *acl.ForbiddenError
	if !errors.As(err, &fe) {
		return false
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error":     "forbidden",
		"rule_kind": fe.Decision.RuleKind,
		"rule_id":   fe.Decision.RuleID,
		"reason":    fe.Decision.Reason,
	})
	return true
}

// --- Settings API ---

// APISettingsData contains all data needed for the settings page.
type APISettingsData struct {
	UserDefaults  APIUserDefaults                `json:"userDefaults"`
	UserPalette   *dataentryconfig.PaletteConfig `json:"userPalette,omitempty"`
	AllProperties []APIPropertyDef               `json:"allProperties"`
	AllRelations  []APIRelationDef               `json:"allRelations"`
	EntityTypes   []string                       `json:"entityTypes"`
	// LogoURL is the cache-busted URL of the user-uploaded sidebar logo,
	// or nil when no logo is set. The SPA reads this on boot to render
	// the sidebar branding.
	LogoURL *string `json:"logoUrl,omitempty"`
}

// APIUserDefaults is the JSON representation of user defaults.
type APIUserDefaults struct {
	Defaults         map[string]string    `json:"defaults"`
	RelationDefaults map[string]string    `json:"relationDefaults"`
	Overrides        []APIDefaultOverride `json:"overrides"`
}

// APIDefaultOverride is the JSON representation of a default override.
type APIDefaultOverride struct {
	Types            []string          `json:"types"`
	Defaults         map[string]string `json:"defaults"`
	RelationDefaults map[string]string `json:"relationDefaults"`
}

// APIPropertyDef describes a property for the settings page.
type APIPropertyDef struct {
	Name   string   `json:"name"`
	Type   string   `json:"type"`
	Values []string `json:"values"`
}

// APIRelationDef describes a relation for the settings page.
type APIRelationDef struct {
	Name       string              `json:"name"`
	Label      string              `json:"label"`
	TargetType string              `json:"targetType"`
	Targets    []APIRelationTarget `json:"targets"`
}

// APIRelationTarget is a possible target for a relation.
type APIRelationTarget struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// handleAPISettingsCRUD routes /api/v1/settings requests based on HTTP method.
func (h *appearanceHandler) handleAPISettingsCRUD(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.handleAPIGetSettings(w, r)
	case http.MethodPut, http.MethodPost:
		h.handleAPISaveSettings(w, r)
	default:
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleAPIGetSettings returns the settings data for the settings page.
//
//nolint:gocognit // assembles the settings response from many independent optional sources; each block guards a distinct setting, with no shared structure to factor out.
func (h *appearanceHandler) handleAPIGetSettings(w http.ResponseWriter, r *http.Request) {
	s := h.schema()
	ud := h.settings.UserDefaults()
	if ud == nil {
		ud = &UserDefaults{}
	}

	// Convert user defaults to API format
	apiDefaults := APIUserDefaults{
		Defaults:         ud.Defaults,
		RelationDefaults: ud.RelationDefaults,
	}
	if apiDefaults.Defaults == nil {
		apiDefaults.Defaults = make(map[string]string)
	}
	if apiDefaults.RelationDefaults == nil {
		apiDefaults.RelationDefaults = make(map[string]string)
	}
	for _, o := range ud.Overrides {
		apiOverride := APIDefaultOverride{
			Types:            o.Types,
			Defaults:         o.Defaults,
			RelationDefaults: o.RelationDefaults,
		}
		if apiOverride.Defaults == nil {
			apiOverride.Defaults = make(map[string]string)
		}
		if apiOverride.RelationDefaults == nil {
			apiOverride.RelationDefaults = make(map[string]string)
		}
		apiDefaults.Overrides = append(apiDefaults.Overrides, apiOverride)
	}

	// Collect all properties across entity types
	propMap := make(map[string]APIPropertyDef)
	for _, entTypeName := range s.Meta.EntityTypes() {
		entDef, ok := s.Meta.GetEntityDef(entTypeName)
		if !ok {
			continue
		}
		for propName, propDef := range entDef.Properties {
			if _, exists := propMap[propName]; !exists {
				propMap[propName] = APIPropertyDef{
					Name:   propName,
					Type:   propDef.Type,
					Values: resolvePropertyValues(propDef, s.Meta),
				}
			} else {
				// Merge values for properties that appear on multiple types
				existing := propMap[propName]
				seen := make(map[string]bool)
				for _, v := range existing.Values {
					seen[v] = true
				}
				for _, v := range resolvePropertyValues(propDef, s.Meta) {
					if !seen[v] {
						existing.Values = append(existing.Values, v)
						seen[v] = true
					}
				}
				propMap[propName] = existing
			}
		}
	}
	allProperties := make([]APIPropertyDef, 0, len(propMap))
	for _, p := range propMap {
		allProperties = append(allProperties, p)
	}

	// Collect all relation types with their targets
	allRelations := make([]APIRelationDef, 0)
	for _, relName := range s.Meta.RelationTypes() {
		relDef, ok := s.Meta.GetRelationDef(relName)
		if !ok {
			continue
		}
		rd := APIRelationDef{
			Name:  relName,
			Label: relDef.Label,
		}
		if len(relDef.To) > 0 {
			rd.TargetType = relDef.To[0]
			for _, targetType := range relDef.To {
				// Route candidates through the read gate + redactor (DEC-ZBI39P):
				// listFromStoreByTypes is ungated, so without Filter this picker
				// leaked every target entity's title — including unreadable ones
				// and hidden display properties (BUG-R9EHKV, the worst surface: no
				// gate at all). Filter drops unreadable targets and redacts the
				// rest so DisplayTitle falls back to the id.
				candidates := listFromStoreByTypes(r.Context(), h.services(), []string{targetType})
				for _, e := range h.viewReader.Filter(r.Context(), candidates) {
					rd.Targets = append(rd.Targets, APIRelationTarget{
						ID:    e.ID,
						Title: s.Meta.DisplayTitle(e.ID, e.Type, e.Properties),
					})
				}
			}
		}
		allRelations = append(allRelations, rd)
	}

	data := APISettingsData{
		UserDefaults:  apiDefaults,
		UserPalette:   h.palette.UserPalette(),
		AllProperties: allProperties,
		AllRelations:  allRelations,
		EntityTypes:   s.Meta.EntityTypes(),
	}
	data.LogoURL = h.logo.URL()

	writeJSON(w, data)
}

// handleAPISaveSettings saves the user defaults from JSON input. The
// settingsService persists and republishes atomically so concurrent readers
// see a coherent snapshot.
func (h *appearanceHandler) handleAPISaveSettings(w http.ResponseWriter, r *http.Request) {
	var input APIUserDefaults
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	// Convert API format to internal UserDefaults
	ud := UserDefaults{
		Defaults:         input.Defaults,
		RelationDefaults: input.RelationDefaults,
	}
	for _, o := range input.Overrides {
		ud.Overrides = append(ud.Overrides, DefaultOverride{
			Types:            o.Types,
			Defaults:         o.Defaults,
			RelationDefaults: o.RelationDefaults,
		})
	}

	// The service persists and republishes the defaults atomically, so a
	// concurrent reader can't observe defaults that disagree with disk.
	if err := h.settings.Save(r.Context(), &ud); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to save settings: "+err.Error())
		return
	}

	writeJSON(w, map[string]bool{"ok": true})
}

// coverage-ignore: HTTP handlers tested via e2e tests

// handleAPIPaletteCRUD routes /api/v1/_palette requests based on HTTP method.
func (h *appearanceHandler) handleAPIPaletteCRUD(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.handleAPIGetPalette(w, r)
	case http.MethodPut, http.MethodPost:
		h.handleAPISavePalette(w, r)
	default:
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleAPIGetPalette returns the current user palette.
func (h *appearanceHandler) handleAPIGetPalette(w http.ResponseWriter, _ *http.Request) {
	p := h.palette.UserPalette()
	if p == nil {
		p = &dataentryconfig.PaletteConfig{}
	}
	writeJSON(w, p)
}

// handleAPISavePalette validates and saves the user palette.
func (h *appearanceHandler) handleAPISavePalette(w http.ResponseWriter, r *http.Request) {
	var input dataentryconfig.PaletteConfig
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	// The service validates, persists, and republishes the resolved palette
	// (against the current project palette) atomically. A validation failure
	// is a 400; a persistence failure is a 500.
	if err := h.palette.Save(r.Context(), h.schema().Cfg.Palette, &input); err != nil {
		if errors.Is(err, errInvalidPalette) {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "failed to save palette: "+err.Error())
		return
	}

	writeJSON(w, map[string]bool{"ok": true})
}
