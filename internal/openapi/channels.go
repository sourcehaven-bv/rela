package openapi

import (
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/realtimecfg"
	"github.com/Sourcehaven-BV/rela/internal/realtimelimits"
)

// Realtime channel paths (TKT-ZU7P13). They are emitted only while a
// realtime.yaml is loaded, and only for the entity types a channel type
// attaches to; the server answers every other channel route 404.

const (
	channelHeadSchema  = "ChannelHead"
	channelSchema      = "Channel"
	channelSessionRef  = "ChannelSession"
	channelListSchema  = "ChannelList"
	messageSchema      = "Message"
	messagePageSchema  = "MessagePage"
	refPropSchema      = "RefProp"
	ephemeralSchema    = "EphemeralMessage"
	ephemeralAccSchema = "EphemeralAccepted"
	channelAuthorRef   = "ChannelAuthor"
	actionAnswerSchema = "MessageActionResponse"
	descriptorSchema   = "ChannelSchema"
)

// addChannelSchemaPath adds `GET /api/v1/_schema/channels`. The route
// exists whether realtime is on or not, so it is always documented; with
// realtime off it lists no channel type.
func addChannelSchemaPath(spec *Spec) {
	spec.Paths["/api/v1/_schema/channels"] = PathItem{
		Get: &Operation{
			OperationID: "getChannelSchema",
			Summary:     "Describe the channel types",
			Description: "Every channel type realtime.yaml declares, with its message types, properties, " +
				"actions and labels. The answer is the same for every caller; what you may do in a channel " +
				"is in its `_actions`. With realtime off the list is empty.",
			Tags: []string{"System"},
			Responses: map[string]Response{
				"200": {
					Description: "The channel types",
					Content:     map[string]MediaType{"application/json": {Schema: Ref(descriptorSchema)}},
				},
			},
		},
	}
	addChannelDescriptorSchema(spec)
}

// addChannelDescriptorSchema adds the ChannelSchema component.
func addChannelDescriptorSchema(spec *Spec) {
	str, b := StringSchema, BooleanSchema
	// A label is the authored label, or the name when none is authored; it
	// is never empty.
	labeled := func(extra map[string]*Schema) *Schema {
		props := map[string]*Schema{"name": str(), "label": {Type: "string",
			Description: "The label from realtime.yaml, or the name when none is set; never empty"}}
		maps.Copy(props, extra)
		return ObjectSchema(props, []string{"name", "label"})
	}
	property := labeled(map[string]*Schema{
		"type": {Type: "string", Enum: []string{
			"string", "markdown", "integer", "boolean", "date", "datetime", "ref", "enum",
		}},
		"required": b(),
		"max":      {Type: "integer", Description: "Most characters (Unicode code points) of a string or markdown value"},
		"enum":     {Type: "string", Description: "The schema.yaml enum type of an enum property"},
		"values":   ArraySchema(str()),
		"to":       {Type: "array", Items: str(), Description: "Entity types a ref property may name"},
	})
	property.Required = append(property.Required, "type")
	messageType := labeled(map[string]*Schema{
		"persist":     {Type: "string", Enum: []string{"log", "ephemeral"}},
		"ttl_seconds": {Type: "integer", Description: "How long an ephemeral message lives"},
		"reply":       {Type: "boolean", Description: "Whether a message of the type may reply to another"},
		"properties":  {Type: "array", Items: property, Description: "Required first, then by name"},
		"actions": ArraySchema(labeled(map[string]*Schema{
			"promote_to": {Type: "string", Description: "The entity type a promoting action creates"},
		})),
	})
	messageType.Required = append(messageType.Required, "persist", "reply", "properties", "actions")
	channelType := labeled(map[string]*Schema{
		"attach_to":   ArraySchema(str()),
		"anchor":      {Type: "string", Enum: []string{"family", "face"}},
		"cardinality": {Type: "string", Enum: []string{"one", "many"}},
		"create_label": {Type: "string",
			Description: "create.label of a cardinality many type; absent when not set"},
		"remove_reason_required": b(),
		"lifecycle": {Type: "array", Items: labeled(nil),
			Description: "The declared lifecycle operations, in the order close, reopen, archive, delete"},
		"message_types": ArraySchema(messageType),
	})
	channelType.Required = append(channelType.Required,
		"attach_to", "anchor", "cardinality", "remove_reason_required", "lifecycle", "message_types")
	spec.Components.Schemas[descriptorSchema] = ObjectSchema(map[string]*Schema{
		"channel_types": ArraySchema(channelType),
	}, []string{"channel_types"})
}

// addChannelPaths adds the channel routes of every entity type in
// entityTypes that some channel type of rt attaches to, and the shared
// channel schemas. A nil rt adds nothing.
func (g *Generator) addChannelPaths(spec *Spec, rt *realtimecfg.Compiled, entityTypes []string) {
	if rt == nil {
		return
	}
	added := false
	for _, typeName := range entityTypes {
		var cts []realtimecfg.ChannelType
		for _, ct := range rt.ChannelTypes() {
			if slices.Contains(ct.AttachTo, typeName) {
				cts = append(cts, ct)
			}
		}
		if len(cts) == 0 {
			continue
		}
		added = true
		def := g.meta.Entities[typeName]
		basePath := "/api/v1/" + def.GetPlural(typeName) + "/{id}/_channels"
		tag := def.Label
		spec.Paths[basePath] = PathItem{
			Parameters: []Parameter{refParam()},
			Get: &Operation{
				OperationID: "listChannels" + capitalize(typeName),
				Summary:     "List the channels on a " + def.Label,
				Description: "Every channel type you may read on the entity, with its channels. A type " +
					"whose read role you lack is left out.",
				Tags: []string{tag},
				Parameters: []Parameter{{
					Name: "state", In: "query", Description: "Only channels in this state",
					Schema: &Schema{Type: "string", Enum: []string{"open", "closed", "archived"}},
				}},
				Responses: channelResponses("200", "The channels", Ref(channelListSchema), "400", "403", "404"),
			},
		}
		for _, ct := range cts {
			addChannelTypePaths(spec, basePath, typeName, tag, ct)
		}
	}
	if added {
		addChannelSchemas(spec)
	}
}

// addChannelTypePaths adds the routes of one channel type under basePath.
func addChannelTypePaths(spec *Spec, basePath, typeName, tag string, ct realtimecfg.ChannelType) {
	typePath := basePath + "/" + ct.Name
	opID := capitalize(typeName) + capitalize(ct.Name)
	headPath := typePath
	params := []Parameter{refParam()}
	if ct.Cardinality == realtimecfg.CardinalityMany {
		spec.Paths[typePath] = PathItem{
			Parameters: params,
			Post: &Operation{
				OperationID: "createChannel" + opID,
				Summary:     "Create a " + ct.Name + " channel",
				Description: "Creates a named channel. The server mints the name.",
				Tags:        []string{tag},
				RequestBody: &RequestBody{Content: jsonContent(ObjectSchema(map[string]*Schema{
					"title": {Type: "string", Description: "At most 200 characters"},
				}, nil))},
				Responses: channelResponses("201", "The new channel", Ref(channelSchema),
					"400", "403", "404", "409", "413", "429"),
			},
		}
		headPath = typePath + "/{name}"
		params = append(params, Parameter{
			Name: "name", In: "path", Required: true, Description: "Channel name", Schema: StringSchema(),
		})
	}
	spec.Paths[headPath] = PathItem{
		Parameters: params,
		Get: &Operation{
			OperationID: "getChannel" + opID,
			Summary:     "Read the head of a " + ct.Name + " channel",
			Description: "The channel, its last message, the live ephemeral messages and what you may do. " +
				"Subscribe to this path on the live event stream; on a frame, fetch messages?after_rev=.",
			Tags:      []string{tag},
			Responses: channelResponses("200", "The channel head", Ref(channelHeadSchema), "403", "404", "422"),
		},
	}
	spec.Paths[headPath+"/messages"] = PathItem{
		Parameters: params,
		Get: &Operation{
			OperationID: "listMessages" + opID,
			Summary:     "List messages of a " + ct.Name + " channel",
			Description: "The newest messages, oldest first. after and before page by seq; after_rev " +
				"returns the messages changed after that rev, edits and removals included. Name at most one.",
			Tags: []string{tag},
			Parameters: []Parameter{
				{Name: "after", In: "query", Description: "Messages after this seq", Schema: IntegerSchema()},
				{Name: "before", In: "query", Description: "Messages before this seq", Schema: IntegerSchema()},
				{Name: "after_rev", In: "query", Description: "Messages changed after this rev", Schema: IntegerSchema()},
				{Name: "limit", In: "query", Description: "Page size, at most the configured page limit",
					Schema: IntegerSchema()},
			},
			Responses: channelResponses("200", "One page of messages", Ref(messagePageSchema),
				"400", "403", "404", "422"),
		},
		Post: &Operation{
			OperationID: "postMessage" + opID,
			Summary:     "Post a message to a " + ct.Name + " channel",
			Description: "A log message is stored and answered 201. An ephemeral message is kept in memory " +
				"for its TTL and answered 202, without a seq.",
			Tags:        []string{tag},
			RequestBody: &RequestBody{Required: true, Content: jsonContent(postBodySchema(ct))},
			Responses: withResponse(
				channelResponses("201", "The stored message", Ref(messageSchema),
					"400", "403", "404", "409", "413", "422", "429"),
				"202", Response{Description: "The ephemeral message was accepted",
					Content: jsonContent(Ref(ephemeralAccSchema))}),
		},
	}
	seqParams := append(slices.Clone(params), Parameter{
		Name: "seq", In: "path", Required: true, Description: "Message seq", Schema: IntegerSchema(),
	})
	spec.Paths[headPath+"/messages/{seq}"] = PathItem{
		Parameters: seqParams,
		Patch: &Operation{
			OperationID: "editMessage" + opID,
			Summary:     "Edit your own message",
			Description: "Replaces the properties of your message. Needs edit_own, and the checks a post needs.",
			Tags:        []string{tag},
			RequestBody: &RequestBody{Required: true, Content: jsonContent(ObjectSchema(map[string]*Schema{
				"props": {Type: "object", AdditionalProperties: &Schema{}},
			}, []string{"props"}))},
			Responses: channelResponses("200", "The edited message", Ref(messageSchema),
				"400", "403", "404", "409", "413", "422", "429"),
		},
		Delete: &Operation{
			OperationID: "deleteMessage" + opID,
			Summary:     "Delete or remove a message",
			Description: "The author deletes their own message (delete_own); a moderator removes any message " +
				"(remove). A moderator who gives a reason removes as a moderator.",
			Tags: []string{tag},
			RequestBody: &RequestBody{Content: jsonContent(ObjectSchema(map[string]*Schema{
				"reason": {Type: "string", Description: "At most 500 characters"},
			}, nil))},
			Responses: channelResponses("200", "The removed message", Ref(messageSchema),
				"400", "403", "404", "409", "413", "429"),
		},
	}
	addLifecyclePaths(spec, headPath, opID, tag, params, ct.Lifecycle)
	addActionPaths(spec, headPath+"/messages/{seq}", opID, tag, seqParams, ct)
}

// addActionPaths adds one `POST …/messages/{seq}/_action/{name}` path per
// action name the channel type's message types declare, and the release
// path `DELETE …/messages/{seq}/_action` when there is any action. An
// action name several message types declare shares one path, whose
// description names each of them. The body is empty or `{}`.
func addActionPaths(spec *Spec, msgPath, opID, tag string, params []Parameter, ct realtimecfg.ChannelType) {
	byName := map[string][]string{}
	for _, typ := range slices.Sorted(maps.Keys(ct.MessageTypes)) {
		for name, a := range ct.MessageTypes[typ].Actions {
			line := "`" + typ + "`"
			if p := a.PromoteTo; p != nil {
				line += ", which creates a " + p.Type
				if p.Relate != "" {
					line += " related to the channel's entity by " + p.Relate
				}
			}
			byName[name] = append(byName[name], line)
		}
	}
	if len(byName) == 0 {
		return
	}
	// The server decodes the body strictly into an empty object.
	body := ObjectSchema(map[string]*Schema{}, nil)
	body.AdditionalProperties = &Schema{Not: &Schema{}}
	errs := []string{"400", "403", "404", "409", "413", "422", "429"}
	for _, name := range slices.Sorted(maps.Keys(byName)) {
		spec.Paths[msgPath+"/_action/"+name] = PathItem{
			Parameters: params,
			Post: &Operation{
				OperationID: "action" + capitalize(name) + opID,
				Summary:     "Run the " + name + " action on a message",
				Description: "Declared on the message types " + strings.Join(byName[name], "; ") + ". " +
					"A message carries one action. A promoting action creates its entity as you, and the " +
					"message shows the action pending until the entity exists. The action is offered in " +
					"the message's `_actions`, and a promoting one only to a caller who may create its " +
					"entity. A server that cannot write the graph answers a promoting action 403 " +
					"action_not_offered. A promote that created its entity answers 200 with warnings when " +
					"a later step failed: promote_relate_failed when the relation to the anchor was not " +
					"written, promote_followup_failed when an automation after the create failed.",
				Tags:        []string{tag},
				RequestBody: &RequestBody{Content: jsonContent(body)},
				Responses: channelResponses("200", "The message after the action, and the promote's warnings",
					Ref(actionAnswerSchema), errs...),
			},
		}
	}
	spec.Paths[msgPath+"/_action"] = PathItem{
		Parameters: params,
		Delete: &Operation{
			OperationID: "releaseAction" + opID,
			Summary:     "Release a pending action",
			Description: "Clears an action that stayed pending after a promote failed, so the message can " +
				"take an action again. Needs message_ops.remove, or the action's own gate when the channel " +
				"type declares no remove. The action must have been pending for " +
				releaseWindow() + ".",
			Tags:        []string{tag},
			RequestBody: &RequestBody{Content: jsonContent(body)},
			Responses: channelResponses("200", "The message without its action", Ref(messageSchema),
				"400", "403", "404", "409", "413", "429"),
		},
	}
}

// releaseWindow is [realtimelimits.PendingRelease] in words, such as
// "15 minutes".
func releaseWindow() string {
	return strconv.Itoa(int(realtimelimits.PendingRelease.Minutes())) + " minutes"
}

// lifecycleDoc is the summary and description of a lifecycle operation's
// path.
type lifecycleDoc struct {
	summary, description string
}

var lifecycleText = map[string]lifecycleDoc{
	realtimecfg.LifecycleClose: {
		summary:     "Close the channel",
		description: "Closes the channel: it keeps its messages and takes no new ones.",
	},
	realtimecfg.LifecycleReopen: {
		summary:     "Reopen the channel",
		description: "Opens a closed or archived channel again.",
	},
	realtimecfg.LifecycleArchive: {
		summary:     "Archive the channel",
		description: "Archives the channel: it keeps its messages and takes no new ones.",
	},
	realtimecfg.LifecycleDelete: {
		summary: "Delete the channel",
		description: "Deletes the channel and its messages. " +
			"A cardinality-one channel starts again at the next post.",
	},
}

// addLifecyclePaths adds one `POST …/_lifecycle/{op}` path per operation
// the channel type declares. The body is empty or `{}`.
func addLifecyclePaths(
	spec *Spec, headPath, opID, tag string, params []Parameter, lc realtimecfg.Lifecycle,
) {
	for _, name := range realtimecfg.LifecycleOpNames() {
		if lc.Op(name) == nil {
			continue
		}
		errs := []string{"400", "403", "404", "409", "413", "422", "429"}
		var responses map[string]Response
		if name == realtimecfg.LifecycleDelete {
			responses = withResponse(channelErrors(errs...), "204", Response{Description: "The channel is deleted"})
		} else {
			responses = channelResponses("200", "The channel after the change", Ref(channelSchema), errs...)
		}
		// The server decodes the body strictly into an empty object.
		body := ObjectSchema(map[string]*Schema{}, nil)
		body.AdditionalProperties = &Schema{Not: &Schema{}}
		doc := lifecycleText[name]
		spec.Paths[headPath+"/_lifecycle/"+name] = PathItem{
			Parameters: params,
			Post: &Operation{
				OperationID: name + "Channel" + opID,
				Summary:     doc.summary,
				Description: doc.description + " The operation is offered in the head's `_actions.lifecycle`.",
				Tags:        []string{tag},
				RequestBody: &RequestBody{Content: jsonContent(body)},
				Responses:   responses,
			},
		}
	}
}

func refParam() Parameter {
	return Parameter{Name: "id", In: "path", Required: true, Description: "Entity ID, or ID@face", Schema: StringSchema()}
}

// channelErrorText describes each error status a channel route answers.
var channelErrorText = map[string]string{
	"400": "Invalid JSON, an unknown field, or an invalid message or query",
	"403": "A role or gate refused; the problem names the rule",
	"404": "Entity not found (the same body as the entity GET), or no such channel type, channel or message",
	"409": "The channel is not open or not in the state the operation starts from, the message was removed, " +
		"a limit, the retention or a concurrent change refused",
	"413": "Request body or a value too large",
	"422": "A ref to an entity that cannot be referenced, or a bare id for a face-anchored type",
	"429": "Rate limited; see Retry-After",
}

// channelResponses is the success response plus the listed error ones.
func channelResponses(code, desc string, schema *Schema, errs ...string) map[string]Response {
	return withResponse(channelErrors(errs...), code, Response{Description: desc, Content: jsonContent(schema)})
}

// channelErrors is the listed error responses.
func channelErrors(errs ...string) map[string]Response {
	out := make(map[string]Response, len(errs)+1)
	for _, e := range errs {
		r := Response{Description: channelErrorText[e], Content: problemContent()}
		if e == "429" {
			r.Headers = map[string]Header{"Retry-After": {Description: "Seconds to wait", Schema: IntegerSchema()}}
		}
		out[e] = r
	}
	return out
}

func withResponse(m map[string]Response, code string, r Response) map[string]Response {
	m[code] = r
	return m
}

// postBodySchema is the post body of ct: one variant per message type,
// told apart by type.
func postBodySchema(ct realtimecfg.ChannelType) *Schema {
	names := make([]string, 0, len(ct.MessageTypes))
	for name := range ct.MessageTypes {
		names = append(names, name)
	}
	slices.Sort(names)
	variants := make([]*Schema, 0, len(names))
	for _, name := range names {
		mt := ct.MessageTypes[name]
		props := propsSchema(mt.PropsSchema())
		body := map[string]*Schema{
			"type":  {Type: "string", Enum: []string{name}},
			"props": props,
		}
		if mt.Persist != realtimecfg.PersistEphemeral {
			body["reply_to"] = &Schema{Type: "integer", Description: "Seq of the top-level message to reply to"}
		}
		top := []string{"type"}
		if len(props.Required) > 0 {
			// A type with a required property cannot be posted without props.
			top = append(top, "props")
		}
		s := ObjectSchema(body, top)
		s.Title = name
		s.Description = mt.Label
		variants = append(variants, s)
	}
	return &Schema{OneOf: variants}
}

// propsSchema converts a message type's props schema, which the MCP tools
// publish too, to an OpenAPI schema.
func propsSchema(ps realtimecfg.PropsSchema) *Schema {
	props := make(map[string]*Schema, len(ps.Properties))
	for name, p := range ps.Properties {
		props[name] = &Schema{
			Type: p.Type, Format: p.Format, Enum: p.Enum, MaxLength: p.MaxLength, Description: p.Description,
		}
	}
	return &Schema{Type: ps.Type, Properties: props, Required: ps.Required, Closed: !ps.AdditionalProperties}
}

// addChannelSchemas adds the schemas the channel routes share.
func addChannelSchemas(spec *Spec) {
	str, i64, b := StringSchema, IntegerSchema, BooleanSchema
	sc := spec.Components.Schemas
	sc[channelAuthorRef] = ObjectSchema(map[string]*Schema{
		"user": str(), "tool": str(), "principal_type": str(),
		"system": {Type: "boolean", Description: "Set on a message a trigger posted"},
	}, []string{"user"})
	sc[refPropSchema] = &Schema{
		Type:        "object",
		Description: "A referenced entity. An entity you cannot read shows only restricted, without its id.",
		Properties:  map[string]*Schema{"id": str(), "type": str(), "title": str(), "restricted": b()},
	}
	sc[channelSchema] = ObjectSchema(map[string]*Schema{
		"anchor": str(), "type": str(), "name": str(),
		"state":      {Type: "string", Enum: []string{"open", "closed", "archived"}},
		"title":      str(),
		"created_by": Ref(channelAuthorRef),
		"created_at": dateTimeSchema(), "closed_at": dateTimeSchema(), "archived_at": dateTimeSchema(),
		"last_seq": i64(), "rev": i64(),
		"session": Ref(channelSessionRef),
	}, []string{"anchor", "type", "state", "last_seq", "rev"})
	sc[channelSessionRef] = channelSessionSchema()
	sc[channelListSchema] = ObjectSchema(map[string]*Schema{
		"anchor": str(),
		"types": ArraySchema(ObjectSchema(map[string]*Schema{
			"type": str(), "label": str(),
			"cardinality": {Type: "string", Enum: []string{"one", "many"}},
			"create":      b(),
			"channels":    ArraySchema(Ref(channelSchema)),
		}, []string{"type", "cardinality", "create", "channels"})),
	}, []string{"anchor", "types"})
	sc[messageSchema] = &Schema{
		Type: "object",
		Description: "A message. A removed message keeps only seq, rev, reply_to, reply_count and removed. " +
			"A ref property renders as a RefProp. A session notice records a transition: its type is " +
			"session, its author is system:realtime with system set, triggered_by is session:<channel type>, " +
			"and its props are from, to, generation and cause (the seq of the message that moved the " +
			"session). A notice cannot be edited or removed (409 session_notice). The author of the message " +
			"that moved the session can no longer edit or delete it (409 session_message); a moderator may " +
			"still remove it.",
		Properties: map[string]*Schema{
			"seq": i64(), "rev": i64(), "type": str(), "author": Ref(channelAuthorRef), "at": dateTimeSchema(),
			"props":    {Type: "object", AdditionalProperties: &Schema{}},
			"reply_to": i64(), "reply_count": i64(), "edited_at": dateTimeSchema(),
			"triggered_by": {Type: "string",
				Description: "trigger:<name> of the trigger that posted the message; absent for a person's"},
			"removed": ObjectSchema(map[string]*Schema{
				"at":     dateTimeSchema(),
				"kind":   {Type: "string", Enum: []string{"author", "moderator"}},
				"by":     Ref(channelAuthorRef),
				"reason": str(),
			}, []string{"at", "kind"}),
			"action": ObjectSchema(map[string]*Schema{
				"name": str(),
				"status": {Type: "string", Enum: []string{"pending", "done"},
					Description: "pending while a promote runs, or after one failed until it is released"},
				"at": dateTimeSchema(), "promoted": Ref(refPropSchema),
			}, []string{"name", "status", "at"}),
			"_actions": {Type: "array", Items: str(),
				Description: "What you may do with the message now, sorted: its actions, and the operations " +
					"edit (PATCH), delete (your own, DELETE), remove (a moderator's DELETE with a reason) and " +
					"release (DELETE …/_action). An edit keeps a restricted ref property it does not name."},
			"edit_until": {Type: "string", Format: "date-time",
				Description: "When edit stops being offered; set while it is offered and the edit window applies"},
			"release_after": {Type: "string", Format: "date-time",
				Description: "When release starts being offered; set while only the pending time keeps it back"},
		},
		Required: []string{"seq", "rev"},
	}
	sc[actionAnswerSchema] = ObjectSchema(map[string]*Schema{
		"message":  Ref(messageSchema),
		"warnings": ArraySchema(Ref("Warning")),
	}, []string{"message"})
	sc[messagePageSchema] = ObjectSchema(map[string]*Schema{
		"channel":        Ref(channelSchema),
		"messages":       ArraySchema(Ref(messageSchema)),
		"has_more":       {Type: "boolean", Description: "The page is full, so there may be more past its cursor"},
		"next_after":     {Type: "integer", Description: "Seq to pass as after; set on an after page"},
		"next_before":    {Type: "integer", Description: "Seq to pass as before; set on the newest and a before page"},
		"next_after_rev": {Type: "integer", Description: "Rev to pass as after_rev; set on the newest and an after_rev page, also when it is 0"},
	}, []string{"channel", "messages", "has_more"})
	sc[ephemeralSchema] = ObjectSchema(map[string]*Schema{
		"id": str(), "type": str(), "author": Ref(channelAuthorRef), "at": dateTimeSchema(),
		"expires_at": dateTimeSchema(), "props": {Type: "object", AdditionalProperties: &Schema{}},
	}, []string{"id", "type", "author", "at", "expires_at"})
	sc[ephemeralAccSchema] = ObjectSchema(map[string]*Schema{
		"id": str(), "expires_at": dateTimeSchema(),
	}, []string{"id", "expires_at"})
	sc[channelHeadSchema] = ObjectSchema(map[string]*Schema{
		"channel":   Ref(channelSchema),
		"last":      Ref(messageSchema),
		"ephemeral": ArraySchema(Ref(ephemeralSchema)),
		"_actions": ObjectSchema(map[string]*Schema{
			"post": ArraySchema(str()), "edit_own": b(), "delete_own": b(), "remove": b(),
			"lifecycle": ArraySchema(str()),
		}, []string{"post", "edit_own", "delete_own", "remove", "lifecycle"}),
	}, []string{"channel", "ephemeral", "_actions"})
}

// channelSessionSchema is the ChannelSession schema.
func channelSessionSchema() *Schema {
	str, i64, b := StringSchema, IntegerSchema, BooleanSchema
	return &Schema{
		Type: "object",
		Description: "The channel's session; absent for a channel type without one. generation counts the " +
			"transitions (0: never moved). stranded marks a stored state realtime.yaml no longer declares; " +
			"label is then the stored name and transitions is empty.",
		Properties: map[string]*Schema{
			"state": str(), "label": str(), "generation": i64(), "entered_at": dateTimeSchema(),
			"final": b(), "stranded": b(),
			"transitions": ArraySchema(ObjectSchema(map[string]*Schema{
				"message_type": str(), "to": str(),
				"by": {Type: "string", Enum: []string{"trigger"},
					Description: "Set when only a trigger's message fires the transition"},
			}, []string{"message_type", "to"})),
		},
		Required: []string{"state", "generation", "entered_at"},
	}
}
