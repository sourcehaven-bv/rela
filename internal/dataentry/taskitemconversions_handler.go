package dataentry

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"strings"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// taskItemConverter is what the task-item endpoint needs of
// [entitymanager.TaskItemConverter]. Every check runs there; the handler
// only maps HTTP onto it.
type taskItemConverter interface {
	Definition(name string) (metamodel.TaskItemConversionDef, bool)
	Preview(ctx context.Context, req entitymanager.TaskItemRequest) (*entitymanager.TaskItemPreview, error)
	Convert(ctx context.Context, req entitymanager.TaskItemRequest) (*entitymanager.TaskItemResult, error)
}

// mentionFunc resolves the display title the principal is served for ref
// under meta, through the request's read gate.
type mentionFunc func(ctx context.Context, meta *metamodel.Metamodel, ref entityPkg.Ref) (v1.Mention, bool)

// viewMention resolves through app's view reader. It reads the reader per
// call because NewApp assigns it after the handler is built. It asks for the
// face named by ref, so a faced entity is titled by the face just written.
func viewMention(app *App) mentionFunc {
	return func(ctx context.Context, meta *metamodel.Metamodel, ref entityPkg.Ref) (v1.Mention, bool) {
		world := worldFromContext(ctx).visibility()
		served := app.viewReader.Resolver().ResolveHeaders(ctx, world, []entityPkg.Ref{ref})
		h, ok := served[ref]
		if !ok || !h.Served() {
			return v1.Mention{}, false
		}
		return buildMention(headerEntity(h.Header), meta), true
	}
}

type taskItemConversionHandler struct {
	conv    taskItemConverter
	schema  func() *Schema
	mention mentionFunc
}

// newTaskItemConversionHandler returns the handler of
// /api/v1/_task_item_conversions/.
//
// Nil: rejected — every collaborator is called on each request.
func newTaskItemConversionHandler(
	conv taskItemConverter, schema func() *Schema, mention mentionFunc,
) (*taskItemConversionHandler, error) {
	if conv == nil || schema == nil || mention == nil {
		return nil, errors.New("dataentry: newTaskItemConversionHandler: converter, schema and mention are required")
	}
	return &taskItemConversionHandler{conv: conv, schema: schema, mention: mention}, nil
}

// taskItemBodyLimit bounds a request body: one line of item text and the
// values of a few required fields.
const taskItemBodyLimit = 64 << 10

type taskItemConversionRequest struct {
	SourceID   string         `json:"source_id"`
	SourceFace string         `json:"source_face,omitempty"`
	Text       string         `json:"text"`
	Properties map[string]any `json:"properties,omitempty"`
}

func (h *taskItemConversionHandler) handleV1TaskItemConversions(w http.ResponseWriter, r *http.Request) {
	snapshot := h.schema()
	if r.Method != http.MethodPost {
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
		return
	}
	name := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/_task_item_conversions/"), "/")
	preflight := strings.HasSuffix(name, "/preflight")
	if preflight {
		name = strings.TrimSuffix(name, "/preflight")
	}
	declared, exists := snapshot.Meta.TaskItemConversions[name]
	if name == "" || strings.Contains(name, "/") || !exists {
		writeV1Error(w, r, http.StatusNotFound, "not_found", "No such task item conversion", name)
		return
	}
	// The client read the offer from this schema snapshot. If the converter
	// holds another one, a reload landed in between.
	if kernelDef, kernelHas := h.conv.Definition(name); !kernelHas || !reflect.DeepEqual(declared, kernelDef) {
		writeV1Error(w, r, http.StatusConflict, "schema_changed",
			"Task item conversion changed; reload and try again", "")
		return
	}
	var body taskItemConversionRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, taskItemBodyLimit)).Decode(&body); err != nil {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_json", "Request body is not valid JSON", err.Error())
		return
	}
	if body.SourceID == "" {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_request", "source_id is required", "")
		return
	}
	req := entitymanager.TaskItemRequest{
		Definition: name,
		SourceID:   body.SourceID,
		SourceFace: body.SourceFace,
		Text:       body.Text,
		Properties: body.Properties,
	}
	if preflight {
		preview, err := h.conv.Preview(r.Context(), req)
		if err != nil {
			writeTaskItemConversionError(w, r, err)
			return
		}
		required := preview.Required
		if required == nil {
			required = []string{}
		}
		writeV1JSON(w, http.StatusOK, v1.TaskItemConversionPreview{TargetType: preview.TargetType, Required: required})
		return
	}
	res, err := h.conv.Convert(r.Context(), req)
	if err != nil {
		writeTaskItemConversionError(w, r, err)
		return
	}
	out := v1.TaskItemConversionResult{
		Definition: res.Definition,
		EntityID:   res.Entity.ID,
		EntityType: res.Entity.Type,
		Face:       res.Entity.Face.String(),
		Item:       declared.ItemMode(),
		Warnings:   res.Create.Warnings,
	}
	if res.FollowUpErr != nil {
		logInternalError(r, "conversion_follow_up_failed", res.FollowUpErr)
		out.Warnings = append(out.Warnings, v1.Warning{
			Code:   warnTaskItemFollowUpFailed,
			Detail: "the entity and its relation were created; a step after the create failed",
		})
	}
	// The title is read back through the read gate, as the editor's other
	// references are: an automation may have hidden the new entity, or its
	// display property, from the caller.
	if m, ok := h.mention(r.Context(), snapshot.Meta, res.Entity.Ref()); ok && !m.Inaccessible {
		out.Title = m.Title
	}
	writeV1JSON(w, http.StatusOK, out)
}

// warnTaskItemFollowUpFailed is the warning for a conversion whose entity and
// relation were committed but whose automations or cascade failed after.
const warnTaskItemFollowUpFailed = "follow_up_failed"

func writeTaskItemConversionError(w http.ResponseWriter, r *http.Request, err error) {
	writeEntityMakingError(w, r, err, entityMakingErrors{
		label:         "Task item conversion",
		sourceChanged: entitymanager.ErrTaskItemSourceChanged,
		input: func(err error) bool {
			return errors.Is(err, entitymanager.ErrUnknownTaskItemConversion) ||
				errors.Is(err, entitymanager.ErrTaskItemInvalid)
		},
	})
}
