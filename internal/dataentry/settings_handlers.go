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
}

// APIIssue is the JSON representation of a single analysis issue.
type APIIssue struct {
	EntityID   string `json:"entityId"`
	EntityType string `json:"entityType"`
	Title      string `json:"title,omitempty"`
	Message    string `json:"message"`
	Severity   string `json:"severity"` // "error" or "warning"
	CheckType  string `json:"checkType"`

	// ScriptError carries the structured Lua-failure envelope for
	// validation script-error rows. Absent (omitempty) on every
	// other row. The frontend uses presence as the discriminator:
	// rows with scriptError open the ScriptErrorDialog instead of
	// navigating to an entity. Same loopback gating as the
	// action-surface envelope (security.AllowFullScriptDetail).
	ScriptError *ScriptErrorEnvelope `json:"scriptError,omitempty"`
}

// writeJSON writes a JSON response with 200 OK status.
func writeJSON(w http.ResponseWriter, data interface{}) {
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
func (a *App) handleAPISettingsCRUD(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		a.handleAPIGetSettings(w, r)
	case http.MethodPut, http.MethodPost:
		a.handleAPISaveSettings(w, r)
	default:
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleAPIGetSettings returns the settings data for the settings page.
func (a *App) handleAPIGetSettings(w http.ResponseWriter, r *http.Request) {
	s := a.State()
	ud := s.UserDefaults
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
				for _, e := range listFromStoreByTypes(r.Context(), a.Services(), []string{targetType}) {
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
		UserPalette:   s.UserPalette,
		AllProperties: allProperties,
		AllRelations:  allRelations,
		EntityTypes:   s.Meta.EntityTypes(),
	}
	data.LogoURL = s.LogoURL()

	writeJSON(w, data)
}

// handleAPISaveSettings saves the user defaults from JSON input. The save
// to disk and the publication of the new AppState happen atomically via
// mutateState so concurrent readers see a coherent snapshot.
func (a *App) handleAPISaveSettings(w http.ResponseWriter, r *http.Request) {
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

	// Save to file under writeMu, then publish a new AppState with the
	// updated UserDefaults via mutateState. The save and the publish
	// are wrapped together so a concurrent reader cannot observe a
	// State whose UserDefaults disagrees with what's on disk.
	var saveErr error
	ctx := r.Context()
	a.mutateState(func(s *AppState) {
		if err := a.userState.saveUserDefaults(ctx, &ud); err != nil {
			saveErr = err
			return
		}
		s.UserDefaults = &ud
	})
	if saveErr != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to save settings: "+saveErr.Error())
		return
	}

	writeJSON(w, map[string]bool{"ok": true})
}

// coverage-ignore: HTTP handlers tested via e2e tests

// handleAPIPaletteCRUD routes /api/v1/_palette requests based on HTTP method.
func (a *App) handleAPIPaletteCRUD(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		a.handleAPIGetPalette(w, r)
	case http.MethodPut, http.MethodPost:
		a.handleAPISavePalette(w, r)
	default:
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleAPIGetPalette returns the current user palette.
func (a *App) handleAPIGetPalette(w http.ResponseWriter, _ *http.Request) {
	p := a.State().UserPalette
	if p == nil {
		p = &dataentryconfig.PaletteConfig{}
	}
	writeJSON(w, p)
}

// handleAPISavePalette validates and saves the user palette.
func (a *App) handleAPISavePalette(w http.ResponseWriter, r *http.Request) {
	var input dataentryconfig.PaletteConfig
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	if err := dataentryconfig.ValidatePalette(&input); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid palette: "+err.Error())
		return
	}

	// Save to file and publish a new AppState atomically so concurrent
	// readers see the new palette via state.Load() rather than
	// observing torn writes through a shared snapshot pointer.
	var saveErr error
	ctx := r.Context()
	a.mutateState(func(s *AppState) {
		if err := a.userState.saveUserPalette(ctx, &input); err != nil {
			saveErr = err
			return
		}
		s.UserPalette = &input
		s.Palette = dataentryconfig.ResolvePalette(s.Cfg.Palette, &input)
	})
	if saveErr != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to save palette: "+saveErr.Error())
		return
	}

	writeJSON(w, map[string]bool{"ok": true})
}
