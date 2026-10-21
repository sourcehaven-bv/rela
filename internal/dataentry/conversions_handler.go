package dataentry

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/affordances"
	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// converter is what the conversion endpoint needs of
// [entitymanager.Converter]. Every check, including the field gate, runs
// there; the handler only maps HTTP onto it.
type converter interface {
	Definition(name string) (metamodel.ConversionDef, bool)
	Preview(ctx context.Context, req entitymanager.ConversionRequest) (*entitymanager.ConversionPreview, error)
	Convert(ctx context.Context, req entitymanager.ConversionRequest) (*entitymanager.ConversionResult, error)
}

type conversionHandler struct {
	conv   converter
	schema func() *Schema
}

// typeChanges are the surfaces that make an entity of another type: copy,
// conversion and task-item conversion. Each has an affordance and a handler.
type typeChanges struct {
	copyOffers       copyOffersFunc
	copies           *copiesHandler
	conversionOffers func(ctx context.Context, entityType, id string) []entitymanager.ConversionOffer
	conversions      *conversionHandler
	// taskItemOffers and taskItems serve the task-item recipes, which make
	// an entity of another type from a line of this one's body.
	taskItemOffers func(ctx context.Context, source *entityPkg.Entity) []entitymanager.TaskItemOffer
	taskItems      *taskItemConversionHandler
}

// wireTypeChanges builds the copy surface and the field-gated converters
// over em, each with the handler that serves it.
func wireTypeChanges(
	em *entitymanager.Manager, edges entitymanager.CopyEdgeGate, schema func() *Schema, mention mentionFunc,
) (typeChanges, error) {
	copyOffers, copies, err := wireCopies(em, edges)
	if err != nil {
		return typeChanges{}, err
	}
	conv, err := entitymanager.NewConverter(entitymanager.FieldGated(em))
	if err != nil {
		return typeChanges{}, err
	}
	h, err := newConversionHandler(conv, schema)
	if err != nil {
		return typeChanges{}, err
	}
	taskItems, err := entitymanager.NewTaskItemConverter(entitymanager.FieldGated(em), edges)
	if err != nil {
		return typeChanges{}, err
	}
	th, err := newTaskItemConversionHandler(taskItems, schema, mention)
	if err != nil {
		return typeChanges{}, err
	}
	return typeChanges{
		copyOffers: copyOffers, copies: copies, conversionOffers: conv.Offers, conversions: h,
		taskItemOffers: taskItems.Offers, taskItems: th,
	}, nil
}

// newConversionHandler returns the handler of /api/v1/_conversions/.
//
// Nil: rejected — a handler without a converter would answer every request
// with a nil dereference.
func newConversionHandler(conv converter, schema func() *Schema) (*conversionHandler, error) {
	if conv == nil || schema == nil {
		return nil, errors.New("dataentry: newConversionHandler: converter and schema are required")
	}
	return &conversionHandler{conv: conv, schema: schema}, nil
}

type conversionRequest struct {
	SourceID      string            `json:"source_id"`
	SourceFace    string            `json:"source_face,omitempty"`
	TargetID      string            `json:"target_id,omitempty"`
	Properties    map[string]any    `json:"properties,omitempty"`
	Content       *string           `json:"content,omitempty"`
	DropRelations []string          `json:"drop_relations,omitempty"`
	Faces         map[string]string `json:"faces,omitempty"`
}

func (h *conversionHandler) handleV1Conversions(w http.ResponseWriter, r *http.Request) {
	snapshot := h.schema()
	if r.Method != http.MethodPost {
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
		return
	}
	name := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/_conversions/"), "/")
	preflight := strings.HasSuffix(name, "/preflight")
	if preflight {
		name = strings.TrimSuffix(name, "/preflight")
	}
	if name == "" || strings.Contains(name, "/") {
		writeV1Error(w, r, http.StatusNotFound, "not_found", "No such conversion", name)
		return
	}
	declared, exists := snapshot.Meta.Conversions[name]
	if !exists {
		writeV1Error(w, r, http.StatusNotFound, "not_found", "No such conversion", name)
		return
	}
	// The client read the definition from this schema snapshot. If the
	// converter holds another one, a reload landed in between.
	if kernelDef, kernelHas := h.conv.Definition(name); !kernelHas || !reflect.DeepEqual(declared, kernelDef) {
		writeV1Error(w, r, http.StatusConflict, "schema_changed", "Conversion definition changed; reload and try again", "")
		return
	}
	var body conversionRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_json", "Request body is not valid JSON", err.Error())
		return
	}
	if body.SourceID == "" {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_request", "source_id is required", "")
		return
	}
	req := entitymanager.ConversionRequest{
		Definition:    name,
		SourceID:      body.SourceID,
		SourceFace:    body.SourceFace,
		TargetID:      body.TargetID,
		Properties:    body.Properties,
		Content:       body.Content,
		DropRelations: body.DropRelations,
		FaceMapping:   body.Faces,
	}
	if preflight {
		preview, err := h.conv.Preview(r.Context(), req)
		if err != nil {
			writeConversionError(w, r, err)
			return
		}
		moves := make([]v1.ConversionMove, 0, len(preview.Moves))
		for _, mv := range preview.Moves {
			moves = append(moves, v1.ConversionMove{SourceFace: mv.SourceFace, TargetFace: mv.TargetFace})
		}
		writeV1JSON(w, http.StatusOK, v1.ConversionPreview{
			TargetType:       preview.TargetType,
			Required:         preview.Required,
			RequiredFaces:    preview.RequiredFaces,
			RelationsToDrop:  preview.RelationsToDrop,
			InvalidRelations: preview.InvalidRelations,
			InvalidFaces:     preview.InvalidFaces,
			TargetFaces:      preview.TargetFaces,
			Moves:            moves,
		})
		return
	}
	res, err := h.conv.Convert(r.Context(), req)
	if err != nil {
		writeConversionError(w, r, err)
		return
	}
	writeV1JSON(w, http.StatusOK, v1.ConversionResult{
		Definition:       res.Definition,
		OldID:            res.OldID,
		EntityID:         res.Entity.ID,
		Face:             res.Entity.Face.String(),
		RelationsDropped: res.RelationsDropped,
	})
}

func writeConversionError(w http.ResponseWriter, r *http.Request, err error) {
	writeEntityMakingError(w, r, err, entityMakingErrors{
		label:         "Conversion",
		sourceChanged: entitymanager.ErrConversionSourceChanged,
		input:         isConversionInputError,
	})
}

// entityMakingErrors names what differs between the endpoints that make an
// entity from another one: conversions and task-item conversions.
type entityMakingErrors struct {
	label         string
	sourceChanged error
	input         func(error) bool
}

// writeEntityMakingError maps an entity-making error onto a response. The
// order is load-bearing: an input error may wrap a denial, which must still
// answer 403.
func writeEntityMakingError(w http.ResponseWriter, r *http.Request, err error, kind entityMakingErrors) {
	var forbidden *acl.ForbiddenError
	var fieldDenied *affordances.FieldWriteError
	var verr *entitymanager.ValidationError
	var kept *entitymanager.KeptEntityError
	switch {
	case errors.As(err, &kept):
		// The caller wrote the entity, so naming it discloses nothing; what
		// failed after it may be a store error, which stays in the log.
		// EntityID names it again in a field a client can read.
		writeInternalProblem(w, r, "conversion_incomplete", v1.Error{
			Title:    kind.label + " was not completed",
			Detail:   kept.Entity.ID + " was created without its relation",
			EntityID: kept.Entity.ID,
		}, err)
	case errors.As(err, &forbidden):
		writeV1Error(w, r, http.StatusForbidden, "forbidden", "Not permitted", forbidden.Decision.Reason)
	case errors.As(err, &fieldDenied):
		writeV1Error(w, r, http.StatusForbidden, "forbidden", "Not permitted", fieldDenied.Error())
	case errors.Is(err, entitymanager.ErrEntityNotFound):
		writeV1Error(w, r, http.StatusNotFound, "not_found", entityNotFoundTitle, "")
	case errors.Is(err, kind.sourceChanged):
		writeV1Error(w, r, http.StatusConflict, "conflict", "Entity changed during conversion; try again", "")
	case errors.Is(err, entitymanager.ErrEntityAlreadyExists):
		writeV1Error(w, r, http.StatusConflict, "conflict", kind.label+" target is already taken", "")
	case kind.input(err):
		writeV1Error(w, r, http.StatusUnprocessableEntity, "conversion_failed", kind.label+" could not be applied", err.Error())
	case errors.Is(err, entitymanager.ErrCardinalityExceeded):
		writeV1Error(w, r, http.StatusUnprocessableEntity, "cardinality_exceeded", kind.label+" target has no room for the relation", err.Error())
	case errors.As(err, &verr) || entitymanager.IsUniqueViolation(err):
		writeV1Error(w, r, http.StatusUnprocessableEntity, "validation_failed", kind.label+" target fails validation", err.Error())
	default:
		writeEntityMakingCreateError(w, r, err, kind)
	}
}

// writeEntityMakingCreateError maps the create refusals the cases of
// [writeEntityMakingError] do not name, such as an illegal state-machine
// entry or a broken ownership rule, through the classifier the create route
// uses. An error it does not classify is a server error.
func writeEntityMakingCreateError(w http.ResponseWriter, r *http.Request, err error, kind entityMakingErrors) {
	switch entitymanager.ClassifyCreateError(err) {
	case entitymanager.CreateErrForbidden:
		writeV1Error(w, r, http.StatusForbidden, "forbidden", "Not permitted", "")
	case entitymanager.CreateErrConflict:
		writeV1Error(w, r, http.StatusConflict, "conflict", kind.label+" conflicts with an existing entity or relation", "")
	case entitymanager.CreateErrFace, entitymanager.CreateErrInvalid:
		writeV1Error(w, r, http.StatusUnprocessableEntity, "conversion_failed", kind.label+" could not be applied", err.Error())
	case entitymanager.CreateErrUnknown:
		writeInternalError(w, r, "conversion_failed", kind.label+" could not be applied", err)
	}
}

func isConversionInputError(err error) bool {
	for _, sentinel := range []error{
		entitymanager.ErrUnknownConversion,
		entitymanager.ErrConversionSourceMismatch,
		entitymanager.ErrConversionFaceFamily,
		entitymanager.ErrConversionInvalid,
	} {
		if errors.Is(err, sentinel) {
			return true
		}
	}
	return false
}
