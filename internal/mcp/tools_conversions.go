package mcp

import (
	"context"
	"errors"
	"fmt"

	mcpgo "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/affordances"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
)

// Converter runs the schema-declared conversions (`conversions:` in
// schema.yaml) for list_conversions and convert_entity. The wiring passes an
// [entitymanager.Converter] over the same manager the entity tools write
// through, so the `convert:` grant, the read checks and the field gate are
// the ones POST /api/v1/_conversions/ applies.
type Converter interface {
	Offers(ctx context.Context, sourceType, sourceID string) []entitymanager.ConversionOffer
	Preview(ctx context.Context, req entitymanager.ConversionRequest) (*entitymanager.ConversionPreview, error)
	Convert(ctx context.Context, req entitymanager.ConversionRequest) (*entitymanager.ConversionResult, error)
}

// conversionHandler serves the conversion tools.
type conversionHandler struct {
	conv    Converter
	store   GraphReader
	watcher Watcher
}

func selConversions(h handlerSet) conversionHandler { return h.convs }

// registerConversionTools registers list_conversions and convert_entity
// when [Deps.Conversions] is set.
func registerConversionTools(s *Server) {
	if s.deps().Conversions == nil {
		return
	}
	addTool(s, toolListConversions(), conversionTool(s, conversionHandler.handleListConversions))
	addTool(s, toolConvertEntity(), conversionTool(s, conversionHandler.handleConvertEntity))
}

// conversionTool binds a conversion handler. A reload can drop the converter
// after the tools were registered; the tools then say so.
func conversionTool(
	s *Server, method func(conversionHandler, context.Context, *mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error),
) mcpgo.ToolHandler {
	return bind(s, selConversions, func(
		h conversionHandler, ctx context.Context, req *mcpgo.CallToolRequest,
	) (*mcpgo.CallToolResult, error) {
		if h.conv == nil {
			return errorResult("conversions are not available on this server"), nil
		}
		return method(h, ctx, req)
	})
}

func toolListConversions() *mcpgo.Tool {
	return newTool("list_conversions",
		withDescription("List the schema-declared conversions offered on an entity: name, label, target "+
			"type, whether you may run it (and why not), the fields the mapping fills, and the target "+
			"faces each source face may become."),
		withString("id", required(), description("Entity ID")),
	)
}

func toolConvertEntity() *mcpgo.Tool {
	return newTool("convert_entity",
		withDescription("Convert an entity to another type with a conversion from list_conversions, as "+
			"the web app's Convert button does. The entity gets a new ID and type; relations are kept, "+
			"remapped or dropped as the conversion declares. Call with dry_run=true to see the target "+
			"fields still required, the face choices and the relations that would be dropped. A "+
			"conversion that drops relations runs only with confirm=true: ask the user first. "+
			"Result: {definition, old_id, id, face, relations_dropped}."),
		withString("id", required(), description("Entity ID, or ID@face to give values for that face's row")),
		withString("conversion", required(), description("Conversion name, as returned by list_conversions")),
		withObject("properties", description("Values for target fields the mapping does not fill")),
		withString("content", description("Markdown body of the converted entity")),
		withString("target_id", description("ID for the converted entity, for a target type with id_type manual")),
		withObject("faces", description("Target face per source face, where a source face has more than one choice")),
		withBoolean("dry_run", description("Preview only (default false)")),
		withBoolean("confirm", description("true once the user has confirmed dropping the listed relations")),
	)
}

// conversionOfferView is one list_conversions entry.
type conversionOfferView struct {
	Name          string              `json:"name"`
	Label         string              `json:"label,omitempty"`
	To            string              `json:"to"`
	Allowed       bool                `json:"allowed"`
	Reason        string              `json:"reason,omitempty"`
	MappedFields  []string            `json:"mapped_fields,omitempty"`
	ContentMapped bool                `json:"content_mapped,omitempty"`
	FaceChoices   map[string][]string `json:"face_choices,omitempty"`
}

func (h conversionHandler) handleListConversions(
	ctx context.Context, request *mcpgo.CallToolRequest,
) (*mcpgo.CallToolResult, error) {
	args := newToolRequest(request)
	id, err := args.RequireString("id")
	if err != nil {
		return errorResult(err.Error()), nil
	}
	id = trimID(id)
	if refused := wholeEntityRef(id); refused != nil {
		return refused, nil
	}
	source, err := h.store.Resolve(ctx, id)
	if err != nil {
		return entityReadFailed("entity", id, err), nil
	}
	offers := h.conv.Offers(ctx, source.Type, source.ID)
	out := make([]conversionOfferView, len(offers))
	for i, o := range offers {
		out[i] = conversionOfferView{
			Name: o.Name, Label: o.Label, To: o.To, Allowed: o.Allowed, Reason: o.Reason,
			MappedFields: o.MappedFields, ContentMapped: o.ContentMapped, FaceChoices: o.FaceChoices,
		}
		if len(o.FaceChoices) == 0 {
			out[i].FaceChoices = nil
		}
	}
	return jsonResult(struct {
		Conversions []conversionOfferView `json:"conversions"`
	}{Conversions: out}), nil
}

// conversionPreviewView is the dry-run and refusal body of convert_entity.
type conversionPreviewView struct {
	TargetType       string               `json:"target_type"`
	Required         []string             `json:"required,omitempty"`
	RequiredFaces    []string             `json:"required_faces,omitempty"`
	InvalidFaces     []string             `json:"invalid_faces,omitempty"`
	TargetFaces      []string             `json:"target_faces,omitempty"`
	RelationsToDrop  []string             `json:"relations_to_drop,omitempty"`
	InvalidRelations []string             `json:"invalid_relations,omitempty"`
	Moves            []conversionMoveView `json:"face_moves,omitempty"`
	Blocked          bool                 `json:"blocked,omitempty"`
	NeedsConfirm     bool                 `json:"needs_confirm,omitempty"`
}

type conversionMoveView struct {
	SourceFace string `json:"source_face"`
	TargetFace string `json:"target_face"`
}

func previewView(p *entitymanager.ConversionPreview) conversionPreviewView {
	v := conversionPreviewView{
		TargetType: p.TargetType, Required: p.Required, RequiredFaces: p.RequiredFaces,
		InvalidFaces: p.InvalidFaces, TargetFaces: p.TargetFaces,
		RelationsToDrop: p.RelationsToDrop, InvalidRelations: p.InvalidRelations,
	}
	for _, mv := range p.Moves {
		v.Moves = append(v.Moves, conversionMoveView{SourceFace: mv.SourceFace, TargetFace: mv.TargetFace})
	}
	v.Blocked = len(p.Required) > 0 || len(p.RequiredFaces) > 0 || len(p.InvalidFaces) > 0 ||
		len(p.InvalidRelations) > 0
	return v
}

// conversionResultView is convert_entity's result.
type conversionResultView struct {
	Definition       string `json:"definition"`
	OldID            string `json:"old_id"`
	ID               string `json:"id"`
	Face             string `json:"face,omitempty"`
	RelationsDropped int    `json:"relations_dropped"`
}

func (h conversionHandler) handleConvertEntity(
	ctx context.Context, request *mcpgo.CallToolRequest,
) (*mcpgo.CallToolResult, error) {
	args := newToolRequest(request)
	req, refused := conversionRequestFrom(args)
	if refused != nil {
		return refused, nil
	}
	addr := req.SourceID
	if req.SourceFace != "" {
		addr += "@" + req.SourceFace
	}
	// Gate the source first: the converter answers "forbidden" for an
	// entity that exists but is hidden, which would confirm it exists.
	if !readable(ctx, h.store, addr) {
		return errorResult("entity not found: " + addr), nil
	}

	preview, err := h.conv.Preview(ctx, req)
	if err != nil {
		return conversionErrorResult(addr, err), nil
	}
	view := previewView(preview)
	if args.GetBool("dry_run", false) {
		return jsonResult(view), nil
	}
	if view.Blocked {
		result := jsonResult(view)
		result.IsError = true
		return result, nil
	}
	if len(preview.RelationsToDrop) > 0 {
		if !args.GetBool("confirm", false) {
			view.NeedsConfirm = true
			result := jsonResult(view)
			result.IsError = true
			return result, nil
		}
		req.DropRelations = preview.RelationsToDrop
	}

	h.watcher.Pause()
	defer h.watcher.Resume()
	res, err := h.conv.Convert(ctx, req)
	if err != nil {
		return conversionErrorResult(addr, err), nil
	}
	return jsonResult(conversionResultView{
		Definition: res.Definition, OldID: res.OldID, ID: res.Entity.ID,
		Face: res.Entity.Face.String(), RelationsDropped: res.RelationsDropped,
	}), nil
}

// conversionRequestFrom reads convert_entity's arguments. A refusal is
// returned as the tool result.
func conversionRequestFrom(args toolRequest) (entitymanager.ConversionRequest, *mcpgo.CallToolResult) {
	var req entitymanager.ConversionRequest
	addr, err := args.RequireString("id")
	if err != nil {
		return req, errorResult(err.Error())
	}
	id, face, err := entity.ParseStateRef(trimID(addr))
	if err != nil {
		return req, errorResult(err.Error())
	}
	req.SourceID = id
	if !face.IsImplicit() {
		req.SourceFace = face.String()
	}
	if req.Definition, err = args.RequireString("conversion"); err != nil {
		return req, errorResult(err.Error())
	}
	if target, tErr := optionalString(args, "target_id"); tErr != nil {
		return req, errorResult(tErr.Error())
	} else if target != nil {
		req.TargetID = trimID(*target)
	}
	if content, cErr := optionalString(args, "content"); cErr != nil {
		return req, errorResult(cErr.Error())
	} else if content != nil {
		req.Content = content
	}
	if raw, present := args.GetArguments()["properties"]; present && raw != nil {
		props, ok := raw.(map[string]any)
		if !ok {
			return req, errorResult(`argument "properties" is not an object`)
		}
		req.Properties = props
	}
	if raw, present := args.GetArguments()["faces"]; present && raw != nil {
		faces, ok := raw.(map[string]any)
		if !ok {
			return req, errorResult(`argument "faces" is not an object`)
		}
		req.FaceMapping = make(map[string]string, len(faces))
		for k, v := range faces {
			s, isString := v.(string)
			if !isString {
				return req, errorResult(fmt.Sprintf("faces[%q] is not a string", k))
			}
			req.FaceMapping[k] = s
		}
	}
	return req, nil
}

// conversionErrorResult answers a refused or failed conversion, in the order
// the HTTP endpoint maps them: an input error may wrap a denial.
func conversionErrorResult(addr string, err error) *mcpgo.CallToolResult {
	var forbidden *acl.ForbiddenError
	var fieldDenied *affordances.FieldWriteError
	switch {
	case errors.As(err, &forbidden):
		return errorResult("not permitted: " + forbidden.Decision.Reason)
	case errors.As(err, &fieldDenied):
		return errorResult("not permitted: " + fieldDenied.Error())
	case errors.Is(err, entitymanager.ErrEntityNotFound):
		return errorResult("entity not found: " + addr)
	case errors.Is(err, entitymanager.ErrConversionSourceChanged):
		return errorResult("conflict: the entity changed during the conversion; nothing was written, retry")
	case errors.Is(err, entitymanager.ErrEntityAlreadyExists):
		return errorResult("conflict: the target ID is already taken")
	}
	return writeErrorResult(err)
}
