package openapi

import (
	"bytes"
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/realtimecfg"
	"github.com/Sourcehaven-BV/rela/internal/realtimelimits"
)

const channelTestRealtime = `
version: 1
channel_types:
  room:
    attach_to: [task]
    read: { from: attached }
    roles:
      member: { from: attached }
    message_types:
      text:
        properties:
          body: { type: string, required: true }
          about: { type: ref, to: [task, note] }
        post: [member]
        actions:
          ack: { roles: [member] }
          accept: { roles: [member], promote_to: { type: note, map: { title: body } } }
      typing: { persist: ephemeral, post: [member] }
    lifecycle:
      close: { roles: [member] }
      delete: { roles: [member] }
  threads:
    attach_to: [task]
    cardinality: many
    read: { from: attached }
    create: { roles: [member] }
    roles:
      member: { from: attached }
    lifecycle:
      archive: { roles: [member] }
    message_types:
      text:
        properties: { body: { type: string } }
        post: [member]
`

func channelTestConfig(t *testing.T) *realtimecfg.Compiled {
	t.Helper()
	c, err := realtimecfg.Parse([]byte(channelTestRealtime), attachmentTestMeta(), nil)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// TestChannelPaths pins the channel routes of an attached type when
// realtime.yaml is loaded, and their absence when it is not (AC15).
func TestChannelPaths(t *testing.T) {
	rt := channelTestConfig(t)

	t.Run("off", func(t *testing.T) {
		for _, load := range []func() *realtimecfg.Compiled{nil, func() *realtimecfg.Compiled { return nil }} {
			g := New(attachmentTestMeta(), Config{Title: "T", Version: "1"})
			g.SetRealtime(load)
			spec := g.Generate()
			for p := range spec.Paths {
				if strings.Contains(p, "_channels") {
					t.Errorf("realtime off, but the spec has %s", p)
				}
			}
			if _, ok := spec.Components.Schemas[channelHeadSchema]; ok {
				t.Error("realtime off, but the spec has the ChannelHead schema")
			}
		}
	})

	g := New(attachmentTestMeta(), Config{Title: "T", Version: "1"})
	g.SetRealtime(func() *realtimecfg.Compiled { return rt })
	spec := g.Generate()

	for _, tc := range []struct {
		path, method string
		codes        []string
	}{
		{"/api/v1/tasks/{id}/_channels", "GET", []string{"200", "400", "403", "404"}},
		{"/api/v1/tasks/{id}/_channels/room", "GET", []string{"200", "403", "404", "422"}},
		{"/api/v1/tasks/{id}/_channels/room/messages", "GET", []string{"200", "400", "403", "404", "422"}},
		{"/api/v1/tasks/{id}/_channels/room/messages", "POST",
			[]string{"201", "202", "400", "403", "404", "409", "413", "422", "429"}},
		{"/api/v1/tasks/{id}/_channels/room/messages/{seq}", "PATCH",
			[]string{"200", "400", "403", "404", "409", "413", "422", "429"}},
		{"/api/v1/tasks/{id}/_channels/room/messages/{seq}", "DELETE",
			[]string{"200", "400", "403", "404", "409", "413", "429"}},
		{"/api/v1/tasks/{id}/_channels/threads", "POST", []string{"201", "400", "403", "404", "409", "413", "429"}},
		{"/api/v1/tasks/{id}/_channels/threads/{name}", "GET", []string{"200", "403", "404", "422"}},
		{"/api/v1/tasks/{id}/_channels/threads/{name}/messages", "POST",
			[]string{"201", "202", "400", "403", "404", "409", "413", "422", "429"}},
		{"/api/v1/tasks/{id}/_channels/room/_lifecycle/close", "POST",
			[]string{"200", "400", "403", "404", "409", "413", "422", "429"}},
		{"/api/v1/tasks/{id}/_channels/room/_lifecycle/delete", "POST",
			[]string{"204", "400", "403", "404", "409", "413", "422", "429"}},
		{"/api/v1/tasks/{id}/_channels/threads/{name}/_lifecycle/archive", "POST",
			[]string{"200", "400", "403", "404", "409", "429"}},
		{"/api/v1/tasks/{id}/_channels/room/messages/{seq}/_action/ack", "POST",
			[]string{"200", "400", "403", "404", "409", "413", "422", "429"}},
		{"/api/v1/tasks/{id}/_channels/room/messages/{seq}/_action/accept", "POST",
			[]string{"200", "400", "403", "404", "409", "413", "422", "429"}},
		{"/api/v1/tasks/{id}/_channels/room/messages/{seq}/_action", "DELETE",
			[]string{"200", "400", "403", "404", "409", "413", "429"}},
	} {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			item, ok := spec.Paths[tc.path]
			if !ok {
				t.Fatalf("spec has no %s", tc.path)
			}
			op := map[string]*Operation{
				"GET": item.Get, "POST": item.Post, "PATCH": item.Patch, "DELETE": item.Delete,
			}[tc.method]
			if op == nil {
				t.Fatalf("%s has no %s operation", tc.path, tc.method)
			}
			requireResponses(t, tc.path, op, tc.codes...)
			if r, ok := op.Responses["429"]; ok {
				if _, ok := r.Headers["Retry-After"]; !ok {
					t.Error("429 does not describe Retry-After")
				}
			}
		})
	}

	for _, p := range []string{"/api/v1/notes/{id}/_channels", "/api/v1/tasks/{id}/_channels/room/{name}",
		"/api/v1/tasks/{id}/_channels/room/_lifecycle/reopen", "/api/v1/tasks/{id}/_channels/threads/{name}/_lifecycle/close",
		"/api/v1/tasks/{id}/_channels/threads/{name}/messages/{seq}/_action"} {
		if _, ok := spec.Paths[p]; ok {
			t.Errorf("spec has %s, which no channel type serves", p)
		}
	}

	release := spec.Paths["/api/v1/tasks/{id}/_channels/room/messages/{seq}/_action"].Delete
	if !strings.Contains(release.Description, releaseWindow()) {
		t.Errorf("release description %q does not quote the release window %q", release.Description, releaseWindow())
	}
	if time.Duration(realtimelimits.PendingRelease.Minutes())*time.Minute != realtimelimits.PendingRelease {
		t.Error("PendingRelease is not whole minutes; releaseWindow would misstate it")
	}
	accept := spec.Paths["/api/v1/tasks/{id}/_channels/room/messages/{seq}/_action/accept"].Post
	for _, code := range []string{"promote_relate_failed", "promote_followup_failed", "action_not_offered"} {
		if !strings.Contains(accept.Description, code) {
			t.Errorf("action description does not document %s", code)
		}
	}

	post := spec.Paths["/api/v1/tasks/{id}/_channels/room/messages"].Post
	body := post.RequestBody.Content["application/json"].Schema
	if len(body.OneOf) != 2 || body.OneOf[0].Title != "text" || body.OneOf[1].Title != "typing" {
		t.Fatalf("post body variants = %+v, want text and typing", body.OneOf)
	}
	text := body.OneOf[0].Properties["props"]
	if got := text.Required; len(got) != 1 || got[0] != "body" {
		t.Errorf("text props required = %v, want [body]", got)
	}
	// The props schema is the one the MCP tools publish: closed, and
	// rendered as additionalProperties false.
	if want, err := json.Marshal(propsSchema(rt.ChannelTypes()[0].MessageTypes["text"].PropsSchema())); err != nil {
		t.Fatal(err)
	} else if got, _ := json.Marshal(text); !bytes.Equal(got, want) ||
		!strings.Contains(string(got), `"additionalProperties":false`) {

		t.Errorf("text props = %s, want %s with additionalProperties false", got, want)
	}
	if _, ok := body.OneOf[1].Properties["reply_to"]; ok {
		t.Error("an ephemeral message type offers reply_to")
	}
	// props is required on the body exactly when its type has a required
	// property.
	if got := body.OneOf[0].Required; len(got) != 2 || got[0] != "type" || got[1] != "props" {
		t.Errorf("text body required = %v, want [type props]", got)
	}
	if got := body.OneOf[1].Required; len(got) != 1 || got[0] != "type" {
		t.Errorf("typing body required = %v, want [type]", got)
	}
	if _, ok := spec.Paths["/api/v1/tasks/{id}/_channels/room/_lifecycle/delete"].Post.Responses["200"]; ok {
		t.Error("delete answers 204, but the spec describes a 200")
	}
	lcBody := spec.Paths["/api/v1/tasks/{id}/_channels/room/_lifecycle/close"].Post.
		RequestBody.Content["application/json"].Schema
	if ap := lcBody.AdditionalProperties; ap == nil || ap.Not == nil {
		t.Errorf("lifecycle body admits fields the server rejects: %+v", lcBody)
	}
	actions := spec.Components.Schemas[channelHeadSchema].Properties["_actions"]
	if _, ok := actions.Properties["lifecycle"]; !ok {
		t.Error("ChannelHead _actions has no lifecycle")
	}
	page := spec.Components.Schemas[messagePageSchema]
	for _, cursor := range []string{"next_after", "next_before", "next_after_rev"} {
		if _, ok := page.Properties[cursor]; !ok {
			t.Errorf("MessagePage has no %s", cursor)
		}
	}
	accept = spec.Paths["/api/v1/tasks/{id}/_channels/room/messages/{seq}/_action/accept"].Post
	if !strings.Contains(accept.Description, "creates a note") {
		t.Errorf("accept description does not name what it promotes to: %q", accept.Description)
	}
	if _, ok := spec.Components.Schemas[messageSchema].Properties["_actions"]; !ok {
		t.Error("Message has no _actions")
	}
	if _, ok := spec.Components.Schemas[messageSchema].Properties["triggered_by"]; !ok {
		t.Error("Message has no triggered_by")
	}
	if _, ok := spec.Components.Schemas[channelAuthorRef].Properties["system"]; !ok {
		t.Error("ChannelAuthor has no system")
	}
	for _, name := range []string{channelHeadSchema, messageSchema, messagePageSchema, refPropSchema, ephemeralAccSchema,
		actionAnswerSchema, "Warning"} {
		if _, ok := spec.Components.Schemas[name]; !ok {
			t.Errorf("spec has no %s schema", name)
		}
	}
}

// TestChannelPaths_FollowReload pins that a new realtime configuration
// shows in the next spec without a metamodel change.
func TestChannelPaths_FollowReload(t *testing.T) {
	var cur atomic.Pointer[realtimecfg.Compiled]
	g := New(attachmentTestMeta(), Config{Title: "T", Version: "1"})
	g.SetRealtime(cur.Load)
	if _, ok := g.Generate().Paths["/api/v1/tasks/{id}/_channels"]; ok {
		t.Fatal("channel paths before realtime.yaml loaded")
	}
	cur.Store(channelTestConfig(t))
	paths := g.Generate().Paths
	if _, ok := paths["/api/v1/tasks/{id}/_channels"]; !ok {
		t.Fatal("no channel paths after realtime.yaml loaded")
	}
	if _, ok := paths["/api/v1/tasks/{id}/_channels/room/messages/{seq}/_action/ack"]; !ok {
		t.Fatal("no action path after realtime.yaml loaded")
	}
	data, err := g.GenerateJSON()
	if err != nil || !strings.Contains(string(data), "_channels") {
		t.Fatalf("JSON spec lacks the channel paths (err %v)", err)
	}
}

// TestOpenAPI_ChannelSession pins that the Channel schema has a session
// property referencing ChannelSession, whose properties are those of
// v1.ChannelSession, and its transitions those of
// v1.ChannelSessionTransition.
func TestOpenAPI_ChannelSession(t *testing.T) {
	rt := channelTestConfig(t)
	g := New(attachmentTestMeta(), Config{Title: "T", Version: "1"})
	g.SetRealtime(func() *realtimecfg.Compiled { return rt })
	sc := g.Generate().Components.Schemas
	session := sc[channelSchema].Properties["session"]
	if session == nil || session.Ref != "#/components/schemas/"+channelSessionRef {
		t.Fatalf("Channel.session = %+v, want a ref to %s", session, channelSessionRef)
	}
	cs := sc[channelSessionRef]
	if cs == nil {
		t.Fatal("no ChannelSession schema")
	}
	if got, want := schemaKeys(cs), jsonNames(reflect.TypeFor[v1.ChannelSession]()); !slices.Equal(got, want) {
		t.Errorf("ChannelSession properties = %v, want %v", got, want)
	}
	tr := cs.Properties["transitions"].Items
	if got, want := schemaKeys(tr), jsonNames(reflect.TypeFor[v1.ChannelSessionTransition]()); !slices.Equal(got, want) {
		t.Errorf("transition properties = %v, want %v", got, want)
	}
	timers := cs.Properties["timers"]
	if timers == nil || timers.Type != "array" || timers.Items == nil ||
		timers.Items.Ref != "#/components/schemas/"+channelTimerRef {

		t.Fatalf("ChannelSession.timers = %+v, want an array of %s", timers, channelTimerRef)
	}
	if d := cs.Properties["due_at"]; d == nil || d.Format != "date-time" {
		t.Errorf("ChannelSession.due_at = %+v, want a date-time", d)
	}
	got, want := schemaKeys(sc[channelTimerRef]), jsonNames(reflect.TypeFor[v1.ChannelSessionTimer]())
	if !slices.Equal(got, want) {
		t.Errorf("ChannelSessionTimer properties = %v, want %v", got, want)
	}
	reminders := cs.Properties["reminders"]
	if reminders == nil || reminders.Type != "array" || reminders.Items == nil ||
		reminders.Items.Ref != "#/components/schemas/"+channelReminderRef {

		t.Fatalf("ChannelSession.reminders = %+v, want an array of %s", reminders, channelReminderRef)
	}
	got, want = schemaKeys(sc[channelReminderRef]), jsonNames(reflect.TypeFor[v1.ChannelSessionReminder]())
	if !slices.Equal(got, want) {
		t.Errorf("ChannelSessionReminder properties = %v, want %v", got, want)
	}
	if d := sc[messageSchema].Description; !strings.Contains(d, "reminder:<channel type>/<reminder name>") ||
		!strings.Contains(d, "reminder_message") {

		t.Errorf("the Message description does not explain reminders: %s", d)
	}
}

// TestOpenAPI_ChannelSessionDescriptor pins AC 9: the descriptor's
// channel_types[].session has the properties of v1.ChannelSessionSchema
// and its state items those of v1.ChannelSessionStateSchema, a session
// transition has guarded, and the head's _actions.transitions is a
// required array of string.
func TestOpenAPI_ChannelSessionDescriptor(t *testing.T) {
	rt := channelTestConfig(t)
	g := New(attachmentTestMeta(), Config{Title: "T", Version: "1"})
	g.SetRealtime(func() *realtimecfg.Compiled { return rt })
	sc := g.Generate().Components.Schemas
	session := sc[descriptorSchema].Properties["channel_types"].Items.Properties["session"]
	if session == nil {
		t.Fatal("no channel_types[].session in the descriptor")
	}
	if got, want := schemaKeys(session), jsonNames(reflect.TypeFor[v1.ChannelSessionSchema]()); !slices.Equal(got, want) {
		t.Errorf("session properties = %v, want %v", got, want)
	}
	states := session.Properties["states"].Items
	if got, want := schemaKeys(states), jsonNames(reflect.TypeFor[v1.ChannelSessionStateSchema]()); !slices.Equal(got, want) {
		t.Errorf("state properties = %v, want %v", got, want)
	}
	if g := sc[channelSessionRef].Properties["transitions"].Items.Properties["guarded"]; g == nil || g.Type != "boolean" {
		t.Errorf("transition guarded = %+v, want a boolean", g)
	}
	actions := sc[channelHeadSchema].Properties["_actions"]
	tr := actions.Properties["transitions"]
	if tr == nil || tr.Type != "array" || tr.Items == nil || tr.Items.Type != "string" {
		t.Fatalf("_actions.transitions = %+v, want an array of string", tr)
	}
	if !slices.Contains(actions.Required, "transitions") {
		t.Errorf("_actions.transitions is not required: %v", actions.Required)
	}
}

func schemaKeys(s *Schema) []string {
	var out []string
	for k := range s.Properties {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

func jsonNames(typ reflect.Type) []string {
	var out []string
	for f := range typ.Fields() {
		name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		out = append(out, name)
	}
	slices.Sort(out)
	return out
}

// TestChannelPaths_Typing pins AC 17: POST …/_typing is documented only
// for a channel type with typing: true, and the head and descriptor
// schemas carry the typing fields of the wire types.
func TestChannelPaths_Typing(t *testing.T) {
	const cfg = `
version: 1
channel_types:
  chat:
    attach_to: [task]
    typing: true
    read: { from: attached }
    roles:
      member: { from: attached }
    message_types:
      text: { properties: { body: { type: string } }, post: [member] }
  topics:
    attach_to: [task]
    cardinality: many
    typing: true
    read: { from: attached }
    create: { roles: [member] }
    roles:
      member: { from: attached }
    message_types:
      text: { properties: { body: { type: string } }, post: [member] }
`
	rt, err := realtimecfg.Parse([]byte(cfg), attachmentTestMeta(), nil)
	if err != nil {
		t.Fatal(err)
	}
	g := New(attachmentTestMeta(), Config{Title: "T", Version: "1"})
	g.SetRealtime(func() *realtimecfg.Compiled { return rt })
	spec := g.Generate()
	for _, p := range []string{"/api/v1/tasks/{id}/_channels/chat/_typing",
		"/api/v1/tasks/{id}/_channels/topics/{name}/_typing"} {
		item, ok := spec.Paths[p]
		if !ok || item.Post == nil {
			t.Fatalf("spec has no POST %s", p)
		}
		requireResponses(t, p, item.Post, "202", "400", "403", "404", "409", "413", "429")
		if _, ok := item.Post.Responses["429"].Headers["Retry-After"]; !ok {
			t.Error("429 does not describe Retry-After")
		}
		body := item.Post.RequestBody.Content["application/json"].Schema
		if ap := body.AdditionalProperties; ap == nil || ap.Not == nil {
			t.Errorf("typing body admits fields the server rejects: %+v", body)
		}
	}

	off := New(attachmentTestMeta(), Config{Title: "T", Version: "1"})
	plain := channelTestConfig(t)
	off.SetRealtime(func() *realtimecfg.Compiled { return plain })
	for p := range off.Generate().Paths {
		if strings.HasSuffix(p, "/_typing") {
			t.Errorf("spec has %s, but no channel type offers typing", p)
		}
	}

	sc := spec.Components.Schemas
	for name, typ := range map[string]reflect.Type{
		typingAccSchema:   reflect.TypeFor[v1.TypingAccepted](),
		typingEntrySchema: reflect.TypeFor[v1.TypingEntry](),
	} {
		if got, want := schemaKeys(sc[name]), jsonNames(typ); !slices.Equal(got, want) {
			t.Errorf("%s properties = %v, want %v", name, got, want)
		}
	}
	head := sc[channelHeadSchema]
	if got, want := schemaKeys(head), jsonNames(reflect.TypeFor[v1.ChannelHead]()); !slices.Equal(got, want) {
		t.Errorf("ChannelHead properties = %v, want %v", got, want)
	}
	actions := head.Properties["_actions"]
	if got, want := schemaKeys(actions), jsonNames(reflect.TypeFor[v1.ChannelActions]()); !slices.Equal(got, want) {
		t.Errorf("ChannelHead _actions properties = %v, want %v", got, want)
	}
	desc := sc[descriptorSchema].Properties["channel_types"].Items
	if _, ok := desc.Properties["typing"]; !ok || !slices.Contains(desc.Required, "typing") {
		t.Error("the channel type descriptor does not require typing")
	}
}
