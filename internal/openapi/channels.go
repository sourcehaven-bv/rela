package openapi

import (
	"slices"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/realtimecfg"
)

// Realtime channel paths (TKT-ZU7P13). They are emitted only while a
// realtime.yaml is loaded, and only for the entity types a channel type
// attaches to; the server answers every other channel route 404.

const (
	channelHeadSchema  = "ChannelHead"
	channelSchema      = "Channel"
	channelListSchema  = "ChannelList"
	messageSchema      = "Message"
	messagePageSchema  = "MessagePage"
	refPropSchema      = "RefProp"
	ephemeralSchema    = "EphemeralMessage"
	ephemeralAccSchema = "EphemeralAccepted"
	channelAuthorRef   = "ChannelAuthor"
)

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
}

// lifecycleText is the summary and description of each lifecycle
// operation's path.
var lifecycleText = map[string][2]string{
	realtimecfg.LifecycleClose:   {"Close the channel", "Closes the channel: it keeps its messages and takes no new ones."},
	realtimecfg.LifecycleReopen:  {"Reopen the channel", "Opens a closed or archived channel again."},
	realtimecfg.LifecycleArchive: {"Archive the channel", "Archives the channel: it keeps its messages and takes no new ones."},
	realtimecfg.LifecycleDelete: {"Delete the channel", "Deletes the channel and its messages. " +
		"A cardinality-one channel starts again at the next post."},
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
		var responses map[string]Response
		errs := []string{"400", "403", "404", "409", "429"}
		if name == realtimecfg.LifecycleDelete {
			responses = channelResponses("200", "", nil, errs...)
			delete(responses, "200")
			responses["204"] = Response{Description: "The channel is deleted"}
		} else {
			responses = channelResponses("200", "The channel after the change", Ref(channelSchema), errs...)
		}
		text := lifecycleText[name]
		spec.Paths[headPath+"/_lifecycle/"+name] = PathItem{
			Parameters: params,
			Post: &Operation{
				OperationID: name + "Channel" + opID,
				Summary:     text[0],
				Description: text[1] + " The operation is offered in the head's `_actions.lifecycle`.",
				Tags:        []string{tag},
				RequestBody: &RequestBody{Content: jsonContent(ObjectSchema(map[string]*Schema{}, nil))},
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
	out := map[string]Response{code: {Description: desc, Content: jsonContent(schema)}}
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
		props := map[string]*Schema{}
		var required []string
		for pname, p := range mt.Properties {
			props[pname] = messagePropSchema(p)
			if p.Required {
				required = append(required, pname)
			}
		}
		slices.Sort(required)
		body := map[string]*Schema{
			"type":  {Type: "string", Enum: []string{name}},
			"props": {Type: "object", Properties: props, Required: required},
		}
		if mt.Persist != realtimecfg.PersistEphemeral {
			body["reply_to"] = &Schema{Type: "integer", Description: "Seq of the top-level message to reply to"}
		}
		top := []string{"type"}
		if len(required) > 0 {
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

// messagePropSchema maps a message property to JSON Schema.
func messagePropSchema(p realtimecfg.Property) *Schema {
	var s *Schema
	switch p.Type {
	case realtimecfg.PropertyInteger:
		s = IntegerSchema()
	case realtimecfg.PropertyBoolean:
		s = BooleanSchema()
	case realtimecfg.PropertyDate:
		s = &Schema{Type: "string", Format: "date"}
	case realtimecfg.PropertyDatetime:
		s = dateTimeSchema()
	case realtimecfg.PropertyRef:
		s = &Schema{Type: "string", Description: "Entity ID; one of the types " + strings.Join(p.To, ", ")}
	case realtimecfg.PropertyEnum:
		s = &Schema{Type: "string", Enum: slices.Clone(p.Values)}
	default:
		s = StringSchema()
	}
	if p.Label != "" && s.Description == "" {
		s.Description = p.Label
	}
	return s
}

// addChannelSchemas adds the schemas the channel routes share.
func addChannelSchemas(spec *Spec) {
	str, i64, b := StringSchema, IntegerSchema, BooleanSchema
	sc := spec.Components.Schemas
	sc[channelAuthorRef] = ObjectSchema(map[string]*Schema{
		"user": str(), "tool": str(), "principal_type": str(),
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
	}, []string{"anchor", "type", "state", "last_seq", "rev"})
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
			"A ref property renders as a RefProp.",
		Properties: map[string]*Schema{
			"seq": i64(), "rev": i64(), "type": str(), "author": Ref(channelAuthorRef), "at": dateTimeSchema(),
			"props":    {Type: "object", AdditionalProperties: &Schema{}},
			"reply_to": i64(), "reply_count": i64(), "edited_at": dateTimeSchema(),
			"removed": ObjectSchema(map[string]*Schema{
				"at":     dateTimeSchema(),
				"kind":   {Type: "string", Enum: []string{"author", "moderator"}},
				"by":     Ref(channelAuthorRef),
				"reason": str(),
			}, []string{"at", "kind"}),
			"action": ObjectSchema(map[string]*Schema{
				"name": str(), "status": str(), "at": dateTimeSchema(), "promoted": Ref(refPropSchema),
			}, []string{"name", "status", "at"}),
		},
		Required: []string{"seq", "rev"},
	}
	sc[messagePageSchema] = ObjectSchema(map[string]*Schema{
		"channel":        Ref(channelSchema),
		"messages":       ArraySchema(Ref(messageSchema)),
		"has_more":       {Type: "boolean", Description: "The page is full, so there may be more past its cursor"},
		"next_after":     {Type: "integer", Description: "Seq to pass as after; set on an after page"},
		"next_before":    {Type: "integer", Description: "Seq to pass as before; set on the newest and a before page"},
		"next_after_rev": {Type: "integer", Description: "Rev to pass as after_rev; set on an after_rev page"},
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
