package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"maps"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
	mcpgo "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/realtime"
	"github.com/Sourcehaven-BV/rela/internal/realtimecfg"
)

// TestChannelTools_RegisteredOnlyWhenEnabled pins AC 1: the channel tools
// register when realtime is on at construction, and not without it.
func TestChannelTools_RegisteredOnlyWhenEnabled(t *testing.T) {
	t.Parallel()
	f := newChannelFixture(t)
	on := toolNames(t, f.client(t, "alice"))
	for _, name := range channelToolNames {
		if on[name] == nil {
			t.Errorf("%s not registered with realtime on", name)
		}
	}

	f.cfg.Store(nil)
	off := toolNames(t, f.client(t, "alice"))
	for _, name := range channelToolNames {
		if off[name] != nil {
			t.Errorf("%s registered with realtime off", name)
		}
	}
	if schema, _ := json.Marshal(off["watch"].InputSchema); strings.Contains(string(schema), toolReadChannel) {
		t.Error("watch offers read_channel with realtime off")
	}

	noChannels := f.deps(t, false)
	noChannels.Channels = ChannelDeps{}
	srv, err := NewServer(noChannels, "test", WithPrincipal(principal.Principal{User: "a", Tool: principal.ToolMCP}))
	if err != nil {
		t.Fatal(err)
	}
	if got := toolNames(t, connect(t, srv)); got["list_channels"] != nil {
		t.Error("channel tools registered without a channel service")
	}

	half := f.deps(t, false)
	half.Channels.Writer = nil
	if _, err := NewServer(half, "test"); err == nil {
		t.Error("NewServer accepted a reader without a writer")
	}
}

// TestChannelTools_OffAfterReload pins that a reload that turns realtime
// off makes the tools answer that channels are not enabled, and that a
// reload bundle without the channel service is refused.
func TestChannelTools_OffAfterReload(t *testing.T) {
	t.Parallel()
	f := newChannelFixture(t)
	srv := f.server(t, "alice", false)
	cs := connect(t, srv)

	f.cfg.Store(nil)
	text, isErr := call(t, cs, toolReadChannel, map[string]any{"id": "REQ-001", "channel_type": "room"})
	if !isErr || text != realtime.OffText {
		t.Errorf("read_channel with realtime off = %q (error %v), want %q", text, isErr, realtime.OffText)
	}

	bare := f.deps(t, false)
	bare.Channels = ChannelDeps{}
	if err := srv.ReloadDeps(bare); err == nil {
		t.Error("ReloadDeps accepted a bundle without the channel service")
	}
	if err := srv.ReloadDeps(f.deps(t, false)); err != nil {
		t.Errorf("ReloadDeps: %v", err)
	}
}

// TestChannelTools_HiddenAnchorTextEqualsShowEntity pins AC 3: for a
// hidden and a missing anchor, every channel tool answers what show_entity
// answers for the id.
func TestChannelTools_HiddenAnchorTextEqualsShowEntity(t *testing.T) {
	t.Parallel()
	f := newChannelFixture(t)
	f.postAsBob(t, f.svc, "DEC-001", "hidden talk")
	cs := f.client(t, "alice")

	for _, id := range []string{"DEC-001", "REQ-404"} {
		want, isErr := call(t, cs, "show_entity", map[string]any{"id": id})
		if !isErr {
			t.Fatalf("show_entity %s succeeded", id)
		}
		addr := map[string]any{"id": id, "channel_type": "room"}
		calls := map[string]map[string]any{
			"list_channels":  {"id": id},
			toolReadChannel:  addr,
			"post_message":   with(addr, map[string]any{"message_type": "text", "props": map[string]any{"body": "x"}}),
			"edit_message":   with(addr, map[string]any{"seq": 1, "props": map[string]any{"body": "x"}}),
			"remove_message": with(addr, map[string]any{"seq": 1}),
			"create_channel": {"id": id, "channel_type": "threads"},
		}
		for tool, args := range calls {
			got, isErr := call(t, cs, tool, args)
			if !isErr || got != want {
				t.Errorf("%s on %s = %q (error %v), want show_entity's %q", tool, id, got, isErr, want)
			}
		}
	}
}

func with(base, extra map[string]any) map[string]any {
	out := maps.Clone(base)
	maps.Copy(out, extra)
	return out
}

// TestChannelTools_DenialsAndUnknownType pins the refusal texts: a read
// denial names its type, an unknown channel type and a type that does not
// attach to the entity answer unknown_channel_type.
func TestChannelTools_DenialsAndUnknownType(t *testing.T) {
	t.Parallel()
	f := newChannelFixture(t)
	tests := []struct {
		name, user, id, typ, prefix string
	}{
		{"read denied", "alice", "REQ-001", "secret", realtime.TypeReadDenied + ": "},
		{"unknown type", "alice", "REQ-001", "nope", realtime.TypeUnknownChannelType + ": "},
		{"type does not attach", "bob", "DEC-001", "secret", realtime.TypeUnknownChannelType + ": "},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cs := f.client(t, tc.user)
			got, isErr := call(t, cs, toolReadChannel, map[string]any{"id": tc.id, "channel_type": tc.typ})
			if !isErr || !strings.HasPrefix(got, tc.prefix) {
				t.Errorf("read_channel = %q (error %v), want prefix %q", got, isErr, tc.prefix)
			}
		})
	}

	got, isErr := call(t, f.client(t, "alice"), "post_message", map[string]any{
		"id": "REQ-001", "channel_type": "room", "message_type": "announce", "props": map[string]any{"body": "x"},
	})
	if !isErr || !strings.HasPrefix(got, realtime.TypePostDenied+": ") {
		t.Errorf("an announce without chat.moderate = %q, want %s", got, realtime.TypePostDenied)
	}
}

// TestChannelTools_BindIdentity pins that the channel tools run
// Deps.Channels.BindIdentity before each call: it binds a caller the
// server's own edge left unbound, and its error fails the call with the
// fixed message.
func TestChannelTools_BindIdentity(t *testing.T) {
	t.Parallel()
	f := newChannelFixture(t)
	tests := []struct {
		name    string
		bind    func(context.Context) (context.Context, error)
		wantErr string
	}{
		{"binds the caller", f.bndl.GatedReads().Rebind, ""},
		{"error fails the call", func(ctx context.Context) (context.Context, error) {
			return ctx, errors.New("resolve failed")
		}, errIdentityUnresolved.Error()},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			d := f.deps(t, false)
			d.BindIdentity = nil
			d.Channels.BindIdentity = tc.bind
			srv, err := NewServer(d, "test",
				WithPrincipal(principal.Principal{User: "alice", Tool: principal.ToolMCP}))
			if err != nil {
				t.Fatalf("NewServer: %v", err)
			}
			srv.logger = slog.New(slog.DiscardHandler)
			got, isErr := call(t, connect(t, srv), toolReadChannel,
				map[string]any{"id": "REQ-001", "channel_type": "room"})
			if tc.wantErr == "" && isErr {
				t.Errorf("read_channel = %q, want success", got)
			}
			if tc.wantErr != "" && (!isErr || got != tc.wantErr) {
				t.Errorf("read_channel = %q (error %v), want %q", got, isErr, tc.wantErr)
			}
		})
	}
}

// TestChannelTools_RestrictedRef pins AC 4: a ref the reader cannot read
// renders restricted with no id, and posting a hidden and a missing ref
// answer the same text.
func TestChannelTools_RestrictedRef(t *testing.T) {
	t.Parallel()
	f := newChannelFixture(t)
	_, err := f.svc.Post(f.asBob(t), realtime.RawAddress{Anchor: "REQ-001", Type: "room"},
		realtime.PostInput{Type: "text", Props: map[string]any{"body": "see", "about": "DEC-001"}})
	if err != nil {
		t.Fatal(err)
	}
	cs := f.client(t, "alice")
	text := mustCall(t, cs, toolReadChannel, map[string]any{"id": "REQ-001", "channel_type": "room"}, nil)
	if strings.Contains(text, "DEC-001") || !strings.Contains(text, `"about":{"restricted":true}`) {
		t.Errorf("read_channel leaks or misses the restricted ref: %s", text)
	}

	postRef := func(ref string) string {
		got, isErr := call(t, cs, "post_message", map[string]any{
			"id": "REQ-001", "channel_type": "room", "message_type": "text",
			"props": map[string]any{"body": "x", "about": ref},
		})
		if !isErr {
			t.Fatalf("post with ref %s succeeded", ref)
		}
		return strings.ReplaceAll(got, ref, "<ref>")
	}
	if hidden, missing := postRef("DEC-001"), postRef("DEC-404"); hidden != missing {
		t.Errorf("hidden ref answers %q, missing ref %q", hidden, missing)
	}
}

// TestChannelTools_RoundTrip drives every tool through a session and pins
// AC 6: every content-bearing result leads with the notice.
func TestChannelTools_RoundTrip(t *testing.T) {
	t.Parallel()
	f := newChannelFixture(t)
	cs := f.client(t, "alice")
	room := map[string]any{"id": "REQ-001", "channel_type": "room"}

	var posted struct {
		Message realtime.MessageView `json:"message"`
	}
	text := mustCall(t, cs, "post_message", with(room, map[string]any{
		"message_type": "text", "props": map[string]any{"body": "hello"},
	}), &posted)
	assertNoticeFirst(t, "post_message", text)

	text = mustCall(t, cs, "post_message", with(room, map[string]any{
		"message_type": "text", "props": map[string]any{"body": "reply"}, "reply_to": posted.Message.Seq,
	}), nil)
	assertNoticeFirst(t, "post_message reply", text)

	text = mustCall(t, cs, "edit_message", with(room, map[string]any{
		"seq": posted.Message.Seq, "props": map[string]any{"body": "edited"},
	}), nil)
	assertNoticeFirst(t, "edit_message", text)

	var read struct {
		realtime.ReadView
	}
	text = mustCall(t, cs, toolReadChannel, room, &read)
	assertNoticeFirst(t, toolReadChannel, text)
	if len(read.Messages) != 2 || read.NextAfterRev == nil || len(read.MessageTypes) == 0 {
		t.Fatalf("read_channel = %s", text)
	}
	for _, mt := range read.MessageTypes {
		raw, err := json.Marshal(mt.Props)
		if err != nil {
			t.Fatal(err)
		}
		var schema jsonschema.Schema
		if err := json.Unmarshal(raw, &schema); err != nil {
			t.Fatalf("%s props_schema: %v", mt.Name, err)
		}
		if _, err := schema.Resolve(nil); err != nil {
			t.Errorf("%s props_schema does not resolve: %v", mt.Name, err)
		}
	}

	var follow struct {
		realtime.ReadView
	}
	mustCall(t, cs, toolReadChannel, with(room, map[string]any{"after_rev": *read.NextAfterRev}), &follow)
	if len(follow.Messages) != 0 || len(follow.MessageTypes) != 0 || *follow.NextAfterRev != *read.NextAfterRev {
		t.Errorf("after_rev at head = %+v", follow.ReadView)
	}

	text = mustCall(t, cs, "remove_message", with(room, map[string]any{"seq": posted.Message.Seq}), nil)
	assertNoticeFirst(t, "remove_message", text)
	if strings.Contains(text, "edited") {
		t.Errorf("remove_message answer carries the removed body: %s", text)
	}

	text = mustCall(t, cs, "create_channel", map[string]any{
		"id": "REQ-001", "channel_type": "threads", "title": "Plan",
	}, nil)
	assertNoticeFirst(t, "create_channel", text)

	var list struct {
		realtime.AnchorChannels
	}
	text = mustCall(t, cs, "list_channels", map[string]any{"id": "REQ-001"}, &list)
	assertNoticeFirst(t, "list_channels", text)
	if len(list.Types) == 0 {
		t.Errorf("list_channels = %s", text)
	}
	text, isErr := call(t, cs, "list_channels", map[string]any{"id": "REQ-001", "state": "gone"})
	if !isErr || !strings.Contains(text, "state") {
		t.Errorf("list_channels with a bad state = %q", text)
	}
}

func assertNoticeFirst(t *testing.T, tool, text string) {
	t.Helper()
	if !noticeFirst(text) {
		t.Errorf("%s result does not lead with the notice: %.120s", tool, text)
	}
}

// TestChannelTools_Arguments pins the argument checks of the channel tools.
func TestChannelTools_Arguments(t *testing.T) {
	t.Parallel()
	f := newChannelFixture(t)
	cs := f.client(t, "alice")
	room := map[string]any{"id": "REQ-001", "channel_type": "room"}
	tests := []struct {
		name, tool string
		args       map[string]any
		want       string
	}{
		{"two cursors", toolReadChannel, with(room, map[string]any{"after": 1, "before": 3}), "at most one"},
		{"negative after_rev", toolReadChannel, with(room, map[string]any{"after_rev": -1}), "after_rev"},
		{"fraction", toolReadChannel, with(room, map[string]any{"before": 1.5}), "before"},
		{"limit too big", toolReadChannel, with(room, map[string]any{"limit": 51}), "limit"},
		{"signals not bool", toolReadChannel, with(room, map[string]any{"signals": "yes"}), "signals"},
		{"no type", toolReadChannel, map[string]any{"id": "REQ-001"}, "channel_type"},
		{"props not object", "post_message", with(room, map[string]any{"message_type": "text", "props": "x"}), "props"},
		{"no seq", "edit_message", with(room, map[string]any{"props": map[string]any{}}), "seq"},
		{"no props", "edit_message", with(room, map[string]any{"seq": 1}), "props"},
		{"reason not string", "remove_message", with(room, map[string]any{"seq": 1, "reason": 3}), "reason"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, isErr := call(t, cs, tc.tool, tc.args)
			if !isErr || !strings.Contains(got, tc.want) {
				t.Errorf("%s = %q (error %v), want it to name %q", tc.tool, got, isErr, tc.want)
			}
		})
	}
}

// TestPostMessage_Signals pins AC 12: a remote server posts a signal and
// answers {ephemeral, id, expires_at}; a stdio server refuses it with
// signals_not_kept, after the post check.
func TestPostMessage_Signals(t *testing.T) {
	t.Parallel()
	f := newChannelFixture(t)
	typing := map[string]any{"id": "REQ-001", "channel_type": "room", "message_type": "typing"}

	var sig signalResult
	mustCall(t, f.client(t, "alice"), "post_message", typing, &sig)
	if !sig.Ephemeral || sig.ID == "" || sig.ExpiresAt == "" {
		t.Errorf("remote signal = %+v", sig)
	}

	stdio := connect(t, f.server(t, "alice", true))
	got, isErr := call(t, stdio, "post_message", typing)
	if !isErr || !strings.HasPrefix(got, realtime.TypeSignalsNotKept+": ") {
		t.Errorf("stdio signal = %q", got)
	}
	cursor := with(typing, map[string]any{"message_type": "cursor"})
	if got, _ := call(t, stdio, "post_message", cursor); !strings.HasPrefix(got, realtime.TypePostDenied) {
		t.Errorf("stdio signal without the post role = %q, want the post denial first", got)
	}

	var read struct{ realtime.ReadView }
	mustCall(t, stdio, toolReadChannel, map[string]any{"id": "REQ-001", "channel_type": "room"}, &read)
	for _, mt := range read.MessageTypes {
		if mt.Persist == "ephemeral" && mt.CanPost {
			t.Errorf("stdio read_channel offers signal type %s", mt.Name)
		}
	}
	tools := toolNames(t, stdio)
	if strings.Contains(tools["post_message"].Description, "ephemeral,id,expires_at") {
		t.Error("stdio post_message describes signals it refuses")
	}
}

// TestChannelTools_RateLimitedText pins that a rate limit answers
// rate_limited with a retry time.
func TestChannelTools_RateLimitedText(t *testing.T) {
	t.Parallel()
	f := newChannelFixture(t)
	cs := f.client(t, "alice")
	slow := map[string]any{"id": "REQ-001", "channel_type": "room", "message_type": "slow", "props": map[string]any{}}
	mustCall(t, cs, "post_message", slow, nil)
	got, isErr := call(t, cs, "post_message", slow)
	if !isErr || !strings.HasPrefix(got, realtime.TypeRateLimited+": ") || !strings.Contains(got, "retry in") {
		t.Errorf("second slow post = %q", got)
	}
}

// TestChannelToolDescriptions_Notice pins that every channel tool and the
// server instructions carry the untrusted-content notice.
func TestChannelToolDescriptions_Notice(t *testing.T) {
	t.Parallel()
	f := newChannelFixture(t)
	tools := toolNames(t, f.client(t, "alice"))
	for _, name := range channelToolNames {
		if !strings.Contains(tools[name].Description, realtime.UntrustedNotice) {
			t.Errorf("%s description lacks the notice", name)
		}
	}
	if !strings.Contains(serverInstructions(f.meta, true), "untrusted user data") {
		t.Error("server instructions lack the channel rule")
	}
	if strings.Contains(serverInstructions(f.meta, false), "Channel tools") {
		t.Error("server instructions mention channels with realtime off")
	}
}

// TestChannelFailed_Internal pins that an unexpected error is answered
// generically, without its text.
func TestChannelFailed_Internal(t *testing.T) {
	t.Parallel()
	res := channelHandler{}.failed(context.Background(), "read", "REQ-001", errors.New("disk on fire"))
	if text := res.Content[0].(*mcpgo.TextContent).Text; !res.IsError || text != channelFailedText {
		t.Errorf("internal failure = %q", text)
	}
}

// TestChannelFailed_NoPrincipalHint pins that only a transport whose
// identity comes from $USER tells the caller to set it.
func TestChannelFailed_NoPrincipalHint(t *testing.T) {
	t.Parallel()
	f := newChannelFixture(t)
	_, err := f.svc.Read(context.Background(), realtime.RawAddress{Anchor: "REQ-001", Type: "room"},
		realtime.ReadRequest{})
	if !realtime.IsNoPrincipal(err) {
		t.Fatalf("read without identity = %v, want no_principal", err)
	}
	for _, local := range []bool{false, true} {
		res := channelHandler{localIdentity: local}.failed(context.Background(), "read", "REQ-001", err)
		text := res.Content[0].(*mcpgo.TextContent).Text
		if got := strings.HasSuffix(text, realtime.NoPrincipalHint); got != local {
			t.Errorf("local identity %v: text %q", local, text)
		}
	}
}

// TestUnavailableChannels pins that every channel tool answers the reason,
// and that the tools stay registered.
func TestUnavailableChannels(t *testing.T) {
	t.Parallel()
	f := newChannelFixture(t)
	deps := f.deps(t, true)
	deps.Channels = UnavailableChannels("acl.yaml cannot be loaded: broken")
	srv, err := NewServer(deps, "test", WithPrincipal(principal.Principal{User: "bob", Tool: principal.ToolMCP}))
	if err != nil {
		t.Fatal(err)
	}
	cs := connect(t, srv)
	addr := map[string]any{"id": "REQ-001", chanTypeArg: "room"}
	for tool, args := range map[string]map[string]any{
		"list_channels":  {"id": "REQ-001"},
		toolReadChannel:  addr,
		"post_message":   {"id": "REQ-001", chanTypeArg: "room", "message_type": "text"},
		"edit_message":   {"id": "REQ-001", chanTypeArg: "room", chanSeqArg: 1, chanPropsArg: map[string]any{}},
		"remove_message": {"id": "REQ-001", chanTypeArg: "room", chanSeqArg: 1},
		"create_channel": {"id": "REQ-001", chanTypeArg: "threads"},
	} {
		text, isErr := call(t, cs, tool, args)
		if !isErr || text != "acl.yaml cannot be loaded: broken" {
			t.Errorf("%s = %q (error %v)", tool, text, isErr)
		}
	}
}

// goldenChannelTools is the realtime fixture's tools/list golden: the
// channel tools and watch. The default golden pins the rest.
const goldenChannelTools = "tools_list_channels.golden.json"

// TestGolden_ChannelToolsList pins the wire contract of the channel tools
// and of watch with read_channel watchable.
func TestGolden_ChannelToolsList(t *testing.T) {
	t.Parallel()
	f := newChannelFixture(t)
	res, err := f.client(t, "alice").ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	keep := map[string]bool{"watch": true}
	for _, name := range channelToolNames {
		keep[name] = true
	}
	var tools []goldenTool
	for _, tool := range res.Tools {
		if !keep[tool.Name] {
			continue
		}
		schema, err := json.Marshal(tool.InputSchema)
		if err != nil {
			t.Fatal(err)
		}
		tools = append(tools, goldenTool{Name: tool.Name, Description: tool.Description, InputSchema: schema})
	}
	sort.Slice(tools, func(i, j int) bool { return tools[i].Name < tools[j].Name })
	got := marshalStable(t, tools)
	if updateGolden {
		writeGolden(t, goldenChannelTools, got)
		return
	}
	if want := readGolden(t, goldenChannelTools); !bytes.Equal(got, normalizeTrailingNewline(want)) {
		t.Errorf("channel tools differ from the committed golden; review, then regenerate with "+
			"UPDATE_GOLDEN=1.\n--- got ---\n%s", got)
	}
}

// incidentArgs addresses the session channel of REQ-001.
var incidentArgs = map[string]any{"id": "REQ-001", "channel_type": "incident"}

func TestReadChannel_IncludesSession(t *testing.T) {
	t.Parallel()
	f := newChannelFixture(t)
	cs := f.client(t, "alice")
	var read struct{ realtime.ReadView }
	mustCall(t, cs, toolReadChannel, incidentArgs, &read)
	s := read.Channel.Session
	if s == nil || s.State != "detected" || s.Generation != 0 || len(s.Transitions) != 1 {
		t.Fatalf("session = %+v", s)
	}
	var room struct{ realtime.ReadView }
	mustCall(t, cs, toolReadChannel, map[string]any{"id": "REQ-001", "channel_type": "room"}, &room)
	if room.Channel.Session != nil {
		t.Errorf("a room has no session, got %+v", room.Channel.Session)
	}
}

func TestListChannels_IncludesSession(t *testing.T) {
	t.Parallel()
	f := newChannelFixture(t)
	cs := f.client(t, "alice")
	mustCall(t, cs, "post_message", with(incidentArgs, map[string]any{"message_type": "contained"}), nil)
	var list struct{ realtime.AnchorChannels }
	mustCall(t, cs, "list_channels", map[string]any{"id": "REQ-001"}, &list)
	for _, tc := range list.Types {
		if tc.Type != "incident" {
			continue
		}
		if len(tc.Channels) != 1 || tc.Channels[0].Session == nil || tc.Channels[0].Session.State != "contained" {
			t.Fatalf("incident channels = %+v", tc.Channels)
		}
		return
	}
	t.Fatalf("list_channels has no incident type: %+v", list)
}

func TestPostMessage_ReturnsSession(t *testing.T) {
	t.Parallel()
	f := newChannelFixture(t)
	cs := f.client(t, "alice")
	var res struct {
		Session *realtime.SessionView `json:"session"`
	}
	mustCall(t, cs, "post_message", with(incidentArgs, map[string]any{
		"message_type": "text", "props": map[string]any{"body": "looking"},
	}), &res)
	if res.Session == nil || res.Session.State != "detected" || res.Session.Generation != 0 {
		t.Fatalf("after a post that moved nothing, session = %+v", res.Session)
	}
	mustCall(t, cs, "post_message", with(incidentArgs, map[string]any{"message_type": "contained"}), &res)
	if res.Session == nil || res.Session.State != "contained" || res.Session.Generation != 1 || !res.Session.Final {
		t.Fatalf("after a post that moved it, session = %+v", res.Session)
	}
	text := mustCall(t, cs, "post_message", map[string]any{
		"id": "REQ-001", "channel_type": "room", "message_type": "text", "props": map[string]any{"body": "x"},
	}, nil)
	if strings.Contains(text, `"session"`) {
		t.Errorf("a room post carries a session: %s", text)
	}
}

// TestReadChannel_Typing pins AC 20: read_channel shows actions.typing and,
// with signals, typing_now without the caller's own entry; signals hold no
// typing entry.
func TestReadChannel_Typing(t *testing.T) {
	t.Parallel()
	f := newChannelFixture(t)
	f.typeAsBob(t, "REQ-001")
	if _, err := f.svc.Post(f.asBob(t), realtime.RawAddress{Anchor: "REQ-001", Type: "chat"},
		realtime.PostInput{Type: "status"}); err != nil {
		t.Fatal(err)
	}
	args := map[string]any{"id": "REQ-001", "channel_type": "chat", "signals": true}
	var alice struct{ realtime.ReadView }
	mustCall(t, f.client(t, "alice"), toolReadChannel, args, &alice)
	if alice.Actions == nil || !alice.Actions.Typing {
		t.Errorf("alice's actions = %+v, want typing", alice.Actions)
	}
	if len(alice.TypingNow) != 1 || alice.TypingNow[0].Author.User != "bob" {
		t.Errorf("alice's typing_now = %+v, want bob", alice.TypingNow)
	}
	if len(alice.Signals) != 1 || alice.Signals[0].Type != "status" {
		t.Errorf("alice's signals = %+v, want only the status signal", alice.Signals)
	}

	var bob struct{ realtime.ReadView }
	mustCall(t, f.client(t, "bob"), toolReadChannel, args, &bob)
	if len(bob.TypingNow) != 0 {
		t.Errorf("bob's typing_now = %+v, want his own entry left out", bob.TypingNow)
	}

	var room struct{ realtime.ReadView }
	mustCall(t, f.client(t, "alice"), toolReadChannel,
		map[string]any{"id": "REQ-001", "channel_type": "room"}, &room)
	if room.Actions == nil || room.Actions.Typing {
		t.Errorf("room actions = %+v, want no typing", room.Actions)
	}
}

// breachYAML is the detected state of the breach-response example on
// requirements. bob holds chat.read, so he is the lead.
const breachYAML = `
version: 1
channel_types:
  breach:
    attach_to: [requirement]
    read: { from: attached }
    roles:
      lead:   { permission: chat.read }
      member: { from: attached }
    message_types:
      text:      { properties: { body: { type: string } }, post: [member] }
      contained: { properties: { note: { type: string } }, post: [lead, member] }
      dismiss:   { properties: { reason: { type: string } }, post: [lead] }
      escalated: { properties: { note: { type: string } }, post: [lead] }
    session:
      initial: detected
      states:
        detected:
          on:
            contained: { to: contained, roles: [lead] }
            dismiss:   { to: dismissed }
            escalated: { to: contained, by: trigger }
        contained: { final: true }
        dismissed: { final: true }
`

// timerYAML is breachYAML with a timer on the detected state.
var timerYAML = strings.Replace(breachYAML, `            escalated: { to: contained, by: trigger }
`, `            escalated: { to: contained, by: trigger }
          after:
            no-containment: { period: 24h, to: contained, label: Not contained within a day }
`, 1)

// useTimers loads timerYAML into f.
func useTimers(t *testing.T, f *channelFixture) {
	t.Helper()
	if timerYAML == breachYAML {
		t.Fatal("timerYAML added no timer")
	}
	compiled, err := realtimecfg.Parse([]byte(timerYAML), f.meta, nil)
	if err != nil {
		t.Fatal(err)
	}
	f.cfg.Store(compiled)
}

// TestReadChannel_IncludesSessionTimers pins TKT-INM3AZ AC 18: read_channel
// shows the current state's timers and the session's due_at.
func TestReadChannel_IncludesSessionTimers(t *testing.T) {
	t.Parallel()
	f := newChannelFixture(t)
	useTimers(t, f)
	cs := f.client(t, "alice")
	args := map[string]any{"id": "REQ-001", "channel_type": "breach"}
	mustCall(t, cs, "post_message", with(args, map[string]any{
		"message_type": "text", "props": map[string]any{"body": "found it"},
	}), nil)
	var read struct{ realtime.ReadView }
	mustCall(t, cs, toolReadChannel, args, &read)
	s := read.Channel.Session
	if s == nil || s.DueAt == nil || len(s.Timers) != 1 {
		t.Fatalf("session = %+v, want a due time and one timer", s)
	}
	tm := s.Timers[0]
	if tm.Name != "no-containment" || tm.To != "contained" || tm.Since != "state" || tm.DueAt == nil ||
		!tm.DueAt.Equal(*s.DueAt) || !s.DueAt.Equal(s.EnteredAt.Add(24*time.Hour)) {

		t.Errorf("timer = %+v, session due %v entered %v", tm, s.DueAt, s.EnteredAt)
	}
}

// TestPostMessage_ReturnsSessionDue pins TKT-INM3AZ AC 18: post_message's
// session carries the due time after the post.
func TestPostMessage_ReturnsSessionDue(t *testing.T) {
	t.Parallel()
	f := newChannelFixture(t)
	useTimers(t, f)
	var res struct {
		Session *realtime.SessionView `json:"session"`
	}
	mustCall(t, f.client(t, "alice"), "post_message", map[string]any{
		"id": "REQ-001", "channel_type": "breach", "message_type": "text", "props": map[string]any{"body": "x"},
	}, &res)
	if res.Session == nil || res.Session.DueAt == nil || len(res.Session.Timers) != 1 {
		t.Fatalf("session = %+v, want a due time", res.Session)
	}
}

// reminderYAML is breachYAML with a reminder in the detected state.
var reminderYAML = strings.Replace(strings.Replace(breachYAML, `      escalated: { properties: { note: { type: string } }, post: [lead] }
`, `      escalated: { properties: { note: { type: string } }, post: [lead] }
      reminder:  { properties: { body: { type: string } }, post: [lead] }
`, 1), `        dismissed: { final: true }
`, `        dismissed: { final: true }
      reminders:
        status-update:
          label: Status update
          in: [detected]
          every: 30m
          first: 10m
          max: 48
          post: { type: reminder, props: { body: "Post a status update" } }
`, 1)

// useReminders loads reminderYAML into f.
func useReminders(t *testing.T, f *channelFixture) {
	t.Helper()
	compiled, err := realtimecfg.Parse([]byte(reminderYAML), f.meta, nil)
	if err != nil {
		t.Fatal(err)
	}
	f.cfg.Store(compiled)
}

// wantReminder fails unless s lists the status-update reminder with count
// 0 and its first slot, ten minutes after the session entered detected.
func wantReminder(t *testing.T, s *realtime.SessionView) {
	t.Helper()
	if s == nil || len(s.Reminders) != 1 {
		t.Fatalf("session = %+v, want one reminder", s)
	}
	r := s.Reminders[0]
	if r.Name != "status-update" || r.Label != "Status update" || r.Every != "30m" || r.Count != 0 ||
		r.Max != 48 || r.DueAt == nil || !r.DueAt.Equal(s.EnteredAt.Add(10*time.Minute)) {

		t.Errorf("reminder = %+v, entered %v", r, s.EnteredAt)
	}
}

// TestReadChannel_IncludesSessionReminders pins TKT-36Z6X3 AC 20:
// read_channel shows the reminders of the current state.
func TestReadChannel_IncludesSessionReminders(t *testing.T) {
	t.Parallel()
	f := newChannelFixture(t)
	useReminders(t, f)
	cs := f.client(t, "alice")
	args := map[string]any{"id": "REQ-001", "channel_type": "breach"}
	mustCall(t, cs, "post_message", with(args, map[string]any{
		"message_type": "text", "props": map[string]any{"body": "found it"},
	}), nil)
	var read struct{ realtime.ReadView }
	mustCall(t, cs, toolReadChannel, args, &read)
	wantReminder(t, read.Channel.Session)
}

// TestPostMessage_ReturnsSessionReminders pins TKT-36Z6X3 AC 20:
// post_message's session carries the reminders after the post.
func TestPostMessage_ReturnsSessionReminders(t *testing.T) {
	t.Parallel()
	f := newChannelFixture(t)
	useReminders(t, f)
	var res struct {
		Session *realtime.SessionView `json:"session"`
	}
	mustCall(t, f.client(t, "alice"), "post_message", map[string]any{
		"id": "REQ-001", "channel_type": "breach", "message_type": "text", "props": map[string]any{"body": "x"},
	}, &res)
	wantReminder(t, res.Session)
}

// TestReadChannel_IncludesTransitions pins AC 7: read_channel's actions
// list the transitions the caller would fire.
func TestReadChannel_IncludesTransitions(t *testing.T) {
	t.Parallel()
	f := newChannelFixture(t)
	compiled, err := realtimecfg.Parse([]byte(breachYAML), f.meta, nil)
	if err != nil {
		t.Fatal(err)
	}
	f.cfg.Store(compiled)
	args := map[string]any{"id": "REQ-001", "channel_type": "breach"}
	for user, want := range map[string][]string{"bob": {"contained", "dismiss"}, "alice": {}} {
		var read struct{ realtime.ReadView }
		mustCall(t, f.client(t, user), toolReadChannel, args, &read)
		if read.Actions == nil || !slices.Equal(read.Actions.Transitions, want) {
			t.Errorf("%s: actions = %+v, want transitions %v", user, read.Actions, want)
		}
	}
}
