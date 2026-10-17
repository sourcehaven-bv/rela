package dataentry

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"reflect"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/affordances"
	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
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

// typeChanges are the two surfaces that make an entity of another type:
// copy and conversion. Each has an affordance and a handler.
type typeChanges struct {
	copyOffers       copyOffersFunc
	copies           *copiesHandler
	conversionOffers func(ctx context.Context, entityType, id string) []entitymanager.ConversionOffer
	conversions      *conversionHandler
}

// wireTypeChanges builds the copy surface and the field-gated converter
// over em, each with the handler that serves it.
func wireTypeChanges(
	em *entitymanager.Manager, edges entitymanager.CopyEdgeGate, schema func() *Schema,
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
	return typeChanges{copyOffers: copyOffers, copies: copies, conversionOffers: conv.Offers, conversions: h}, nil
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
	var forbidden *acl.ForbiddenError
	if errors.As(err, &forbidden) {
		writeV1Error(w, r, http.StatusForbidden, "forbidden", "Not permitted", forbidden.Decision.Reason)
		return
	}
	var fieldDenied *affordances.FieldWriteError
	if errors.As(err, &fieldDenied) {
		writeV1Error(w, r, http.StatusForbidden, "forbidden", "Not permitted", fieldDenied.Error())
		return
	}
	if errors.Is(err, entitymanager.ErrEntityNotFound) {
		writeV1Error(w, r, http.StatusNotFound, "not_found", entityNotFoundTitle, "")
		return
	}
	if errors.Is(err, entitymanager.ErrConversionSourceChanged) {
		writeV1Error(w, r, http.StatusConflict, "conflict", "Entity changed during conversion; try again", "")
		return
	}
	if errors.Is(err, entitymanager.ErrEntityAlreadyExists) {
		writeV1Error(w, r, http.StatusConflict, "conflict", "Conversion target is already taken", "")
		return
	}
	if isConversionInputError(err) {
		writeV1Error(w, r, http.StatusUnprocessableEntity, "conversion_failed", "Conversion could not be applied", err.Error())
		return
	}
	var verr *entitymanager.ValidationError
	if errors.As(err, &verr) || entitymanager.IsUniqueViolation(err) {
		writeV1Error(w, r, http.StatusUnprocessableEntity, "validation_failed", "Conversion target fails validation", err.Error())
		return
	}
	slog.Error("dataentry: conversion failed", "err", err)
	writeV1Error(w, r, http.StatusInternalServerError, "conversion_failed", "Conversion could not be applied", "check server logs")
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
