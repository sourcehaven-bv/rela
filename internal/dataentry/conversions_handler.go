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
	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
)

type conversionHandler struct {
	manager        *entitymanager.Manager
	validateFields func(context.Context, *entity.Entity, map[string]any) error
	schema         func() *Schema
}

func newConversionHandler(m *entitymanager.Manager) *conversionHandler {
	return &conversionHandler{manager: m}
}

type conversionRequest struct {
	SourceID      string            `json:"source_id"`
	SourceFace    string            `json:"source_face,omitempty"`
	TargetID      string            `json:"target_id,omitempty"`
	Properties    map[string]any    `json:"properties,omitempty"`
	Content       *string           `json:"content,omitempty"`
	DropRelations []string          `json:"drop_relations,omitempty"`
	Faces         map[string]string `json:"faces,omitempty"`
	Preflight     bool              `json:"preflight,omitempty"`
}

func (h *conversionHandler) handleV1Conversions(w http.ResponseWriter, r *http.Request) {
	var snapshot *Schema
	if h.schema != nil {
		snapshot = h.schema()
	}
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
	if snapshot != nil {
		declared, exists := snapshot.Meta.Conversions[name]
		if !exists {
			writeV1Error(w, r, http.StatusNotFound, "not_found", "No such conversion", name)
			return
		}
		kernelDef, kernelHas := entitymanager.ConversionDefinition(h.manager, name)
		if !kernelHas || !reflect.DeepEqual(declared, kernelDef) {
			writeV1Error(w, r, http.StatusConflict, "schema_changed", "Conversion definition changed; reload and try again", "")
			return
		}
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
	preview, err := entitymanager.PreviewConversion(r.Context(), h.manager, req)
	if err != nil {
		writeConversionError(w, r, err)
		return
	}
	if h.validateFields != nil && len(body.Properties) > 0 {
		id := body.TargetID
		if id == "" {
			id = "CONVERSION-PREVIEW"
		}
		candidate := &entity.Entity{ID: id, Type: preview.TargetType, Properties: body.Properties}
		if validationErr := h.validateFields(r.Context(), candidate, body.Properties); validationErr != nil {
			writeV1Error(w, r, http.StatusForbidden, "forbidden", "Not permitted", validationErr.Error())
			return
		}
	}
	if preflight {
		writeV1JSON(w, http.StatusOK, v1.ConversionPreview{
			TargetType:       preview.TargetType,
			Required:         preview.Required,
			RequiredFaces:    preview.RequiredFaces,
			RelationsToDrop:  preview.RelationsToDrop,
			InvalidRelations: preview.InvalidRelations,
			InvalidFaces:     preview.InvalidFaces,
			TargetFaces:      preview.TargetFaces,
		})
		return
	}
	res, err := entitymanager.ConvertEntity(r.Context(), h.manager, req)
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
	if errors.Is(err, entitymanager.ErrEntityNotFound) {
		writeV1Error(w, r, http.StatusNotFound, "not_found", entityNotFoundTitle, "")
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
	if errors.As(err, &verr) {
		writeV1Error(w, r, http.StatusUnprocessableEntity, "validation_failed", "Conversion target fails validation", verr.Error())
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
