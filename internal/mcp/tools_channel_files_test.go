package mcp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	mcpgo "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Sourcehaven-BV/rela/internal/attachment"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/realtimecfg"
)

// filesYAML is channelTestYAML with a file property on room's text.
var filesYAML = strings.Replace(channelTestYAML,
	"          about: { type: ref, to: [requirement, decision] }\n",
	"          about: { type: ref, to: [requirement, decision] }\n          shot: { type: file, max: 2 }\n", 1)

func newFilesFixture(t *testing.T) *channelFixture {
	t.Helper()
	f := newChannelFixture(t)
	compiled, err := realtimecfg.Parse([]byte(filesYAML), f.meta, nil)
	if err != nil {
		t.Fatal(err)
	}
	f.cfg.Store(compiled)
	return f
}

func b64(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }

// post_message takes base64 files; read_message_file returns one by the id
// in the message's files.
func TestChannelFiles_PostAndRead(t *testing.T) {
	f := newFilesFixture(t)
	cs := f.client(t, "alice")
	room := map[string]any{"id": "REQ-001", "channel_type": "room"}
	var got struct {
		Message struct {
			Seq   int64 `json:"seq"`
			Files []struct {
				ID, Property, Name, MIME string
				Size                     int64
			} `json:"files"`
		} `json:"message"`
	}
	mustCall(t, cs, "post_message", with(room, map[string]any{
		"message_type": "text", "props": map[string]any{"body": "see"},
		"files": []any{map[string]any{"property": "shot", "file_name": "notes.txt", "content": b64("hello file")}},
	}), &got)
	if len(got.Message.Files) != 1 {
		t.Fatalf("files = %+v", got.Message.Files)
	}
	file := got.Message.Files[0]
	if file.Name != "notes.txt" || file.Property != "shot" || file.Size != int64(len("hello file")) {
		t.Fatalf("file = %+v", file)
	}

	res, err := cs.CallTool(context.Background(), &mcpgo.CallToolParams{Name: toolReadMessageFile,
		Arguments: mustJSON(t, with(room, map[string]any{"seq": got.Message.Seq, "file_id": file.ID}))})
	if err != nil || res.IsError {
		t.Fatalf("read_message_file: %v %+v", err, res)
	}
	if text, ok := res.Content[0].(*mcpgo.TextContent); !ok || text.Text != "hello file" {
		t.Fatalf("content = %#v", res.Content[0])
	}

	// carol reads the channel and the file; a wrong id reads as missing.
	carol := f.client(t, "carol")
	text, isErr := call(t, carol, toolReadMessageFile,
		with(room, map[string]any{"seq": got.Message.Seq, "file_id": strings.Repeat("a", len(file.ID))}))
	if !isErr || !strings.Contains(text, "file not found") {
		t.Fatalf("wrong id = %q (error %v)", text, isErr)
	}
}

func TestChannelFiles_PostRefused(t *testing.T) {
	f := newFilesFixture(t)
	cs := f.client(t, "alice")
	room := map[string]any{"id": "REQ-001", "channel_type": "room", "message_type": "text",
		"props": map[string]any{"body": "x"}}
	file := func(extra map[string]any) map[string]any {
		return with(map[string]any{"property": "shot", "file_name": "a.txt", "content": b64("x")}, extra)
	}
	for _, tc := range []struct {
		name  string
		files any
		want  string
	}{
		{"not an array", "x", "is not an array"},
		{"not an object", []any{"x"}, "not an object"},
		{"bad base64", []any{file(map[string]any{"content": "!!"})}, "not valid base64"},
		{"unknown key", []any{file(map[string]any{"extra": 1})}, "unknown key"},
		{"no name", []any{file(map[string]any{"file_name": ""})}, "file_name"},
		{"too many", []any{file(nil), file(nil), file(nil), file(nil), file(nil)}, "at most 4"},
		{"over property max", []any{file(nil), file(map[string]any{"file_name": "b.txt"}),
			file(map[string]any{"file_name": "c.txt"})}, "at most 2"},
		{"undeclared property", []any{file(map[string]any{"property": "body"})}, "no file property"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			text, isErr := call(t, cs, "post_message", with(room, map[string]any{"files": tc.files}))
			if !isErr || !strings.Contains(text, tc.want) {
				t.Fatalf("post = %q (error %v), want %q", text, isErr, tc.want)
			}
		})
	}
}

func mustJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// A post with files shares attach_file's upload slots; one without files
// takes none.
func TestChannelFiles_BusyUploads(t *testing.T) {
	f := newFilesFixture(t)
	deps := f.deps(t, false)
	slots := attachment.NewLimiter(1)
	release, ok := slots.TryAcquire()
	if !ok {
		t.Fatal("no slot")
	}
	defer release()
	deps.Attachments.Uploads = slots
	srv, err := NewServer(deps, "test", WithPrincipal(principal.Principal{User: "alice", Tool: principal.ToolMCP}))
	if err != nil {
		t.Fatal(err)
	}
	srv.logger = slog.New(slog.DiscardHandler)
	cs := connect(t, srv)
	post := map[string]any{"id": "REQ-001", "channel_type": "room", "message_type": "text",
		"props": map[string]any{"body": "x"}}
	text, isErr := call(t, cs, "post_message", with(post, map[string]any{
		"files": []any{map[string]any{"property": "shot", "file_name": "a.txt", "content": b64("x")}}}))
	if !isErr || !strings.Contains(text, "too many uploads") {
		t.Fatalf("post with files = %q (error %v), want busy", text, isErr)
	}
	if text, isErr := call(t, cs, "post_message", post); isErr {
		t.Fatalf("post without files = %q", text)
	}
}
