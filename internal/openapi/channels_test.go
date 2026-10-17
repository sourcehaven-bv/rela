package openapi

import (
	"encoding/json"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/realtimecfg"
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
		"/api/v1/tasks/{id}/_channels/room/_lifecycle/reopen", "/api/v1/tasks/{id}/_channels/threads/{name}/_lifecycle/close"} {
		if _, ok := spec.Paths[p]; ok {
			t.Errorf("spec has %s, which no channel type serves", p)
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
	} else if got, _ := json.Marshal(text); string(got) != string(want) ||
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
	for _, name := range []string{channelHeadSchema, messageSchema, messagePageSchema, refPropSchema, ephemeralAccSchema} {
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
	if _, ok := g.Generate().Paths["/api/v1/tasks/{id}/_channels"]; !ok {
		t.Fatal("no channel paths after realtime.yaml loaded")
	}
	data, err := g.GenerateJSON()
	if err != nil || !strings.Contains(string(data), "_channels") {
		t.Fatalf("JSON spec lacks the channel paths (err %v)", err)
	}
}
