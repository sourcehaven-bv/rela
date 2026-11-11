package mcp

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strconv"

	mcpgo "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Sourcehaven-BV/rela/internal/realtime"
)

// Channel file tool names and arguments.
const (
	toolReadMessageFile = "read_message_file"
	chanFilesArg        = "files"
	chanFileIDArg       = "file_id"
)

// ChannelFileOpener backs read_message_file. The wiring site passes a
// *realtime.FileOpener, which runs the channel's read check.
type ChannelFileOpener interface {
	OpenFile(ctx context.Context, addr realtime.RawAddress, seq int64, id string) (realtime.FileDownload, error)
}

func (u unavailableChannels) OpenFile(
	context.Context, realtime.RawAddress, int64, string,
) (realtime.FileDownload, error) {
	return realtime.FileDownload{}, u.err
}

func filesArgDescription() string {
	return "Files to post with the message, for a message type with file properties (message_types[].files " +
		"on read_channel lists them; props_schema does not). Each item is " +
		`{"property": "<file property>", "file_name": "<name with extension>", "content": "<base64>"}. ` +
		"At most " + strconv.Itoa(realtime.MaxFilesPerMessage) + " files and " +
		strconv.Itoa(realtime.MaxMessageFileBytes) + " bytes together; each property's max, max_size " +
		"and accept apply, and a configured scan runs."
}

func toolReadMessageFileDef() *mcpgo.Tool {
	return newTool(toolReadMessageFile, append(withChannelAddress(true),
		withDescription(fmt.Sprintf("Read one file of a channel message, by the id in the message's files. "+
			"Raster images are returned as image content, UTF-8 text as text, and anything else as a base64 "+
			"blob. Files over %d bytes are refused. ", MaxInlineReadBytes)+realtime.UntrustedNotice),
		withNumber(chanSeqArg, required(), description("Message seq")),
		withString(chanFileIDArg, required(), description("File id, from the message's files")),
	)...)
}

// uploadSlots bounds concurrent uploads; *attachment.Limiter satisfies it.
type uploadSlots interface {
	TryAcquire() (release func(), ok bool)
}

// hasFilesArg reports whether args carries a non-empty files argument.
func hasFilesArg(args toolRequest) bool {
	items, ok := args.GetArguments()[chanFilesArg].([]any)
	return ok && len(items) > 0
}

// filesArg reads the files argument of post_message. The service checks
// the properties, counts, sizes and content.
//
// Nil: returned when the argument is absent.
func filesArg(args toolRequest) ([]realtime.FileInput, error) {
	raw, present := args.GetArguments()[chanFilesArg]
	if !present || raw == nil {
		return nil, nil
	}
	items, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("argument %q is not an array", chanFilesArg)
	}
	if len(items) > realtime.MaxFilesPerMessage {
		return nil, fmt.Errorf("%s: at most %d files", chanFilesArg, realtime.MaxFilesPerMessage)
	}
	out := make([]realtime.FileInput, 0, len(items))
	total := 0
	for i, item := range items {
		f, err := fileItem(item)
		if err != nil {
			return nil, fmt.Errorf("%s[%d]: %w", chanFilesArg, i, err)
		}
		if total += len(f.Data); total > realtime.MaxMessageFileBytes {
			return nil, fmt.Errorf("%s: over %d bytes together", chanFilesArg, realtime.MaxMessageFileBytes)
		}
		out = append(out, f)
	}
	return out, nil
}

func fileItem(item any) (realtime.FileInput, error) {
	m, ok := item.(map[string]any)
	if !ok {
		return realtime.FileInput{}, errors.New("not an object")
	}
	for k := range m {
		if k != "property" && k != "file_name" && k != "content" {
			return realtime.FileInput{}, fmt.Errorf("unknown key %q", k)
		}
	}
	var f realtime.FileInput
	for _, field := range []struct {
		key string
		dst *string
	}{{"property", &f.Property}, {"file_name", &f.Name}} {
		s, isStr := m[field.key].(string)
		if !isStr || s == "" {
			return realtime.FileInput{}, fmt.Errorf("%s: a non-empty string", field.key)
		}
		*field.dst = s
	}
	content, ok := m["content"].(string)
	if !ok {
		return realtime.FileInput{}, errors.New("content: a base64 string")
	}
	if len(content) > base64.StdEncoding.EncodedLen(realtime.MaxMessageFileBytes) {
		return realtime.FileInput{}, fmt.Errorf("content: over %d bytes", realtime.MaxMessageFileBytes)
	}
	data, err := base64.StdEncoding.DecodeString(content)
	if err != nil {
		return realtime.FileInput{}, fmt.Errorf("content is not valid base64: %w", err)
	}
	f.Data = data
	return f, nil
}

func (h channelHandler) handleReadFile(ctx context.Context, args toolRequest) *mcpgo.CallToolResult {
	addr, seq, err := messageAddress(args)
	if err != nil {
		return errorResult(err.Error())
	}
	id, err := args.RequireString(chanFileIDArg)
	if err != nil {
		return errorResult(err.Error())
	}
	got, err := h.files.OpenFile(ctx, addr, seq, id)
	if err != nil {
		return h.failed(ctx, "read file", addr.Anchor, err)
	}
	defer got.Body.Close()
	if got.File.Size > MaxInlineReadBytes {
		return errorResult(fmt.Sprintf("file %s is larger than the %d-byte inline read limit",
			got.File.Name, MaxInlineReadBytes))
	}
	data, err := io.ReadAll(io.LimitReader(got.Body, MaxInlineReadBytes+1))
	if err != nil {
		return h.failed(ctx, "read file", addr.Anchor, fmt.Errorf("mcp: read channel file: %w", err))
	}
	if len(data) > MaxInlineReadBytes {
		return errorResult(fmt.Sprintf("file %s is larger than the %d-byte inline read limit",
			got.File.Name, MaxInlineReadBytes))
	}
	return &mcpgo.CallToolResult{Content: []mcpgo.Content{
		attachmentContent(addr.Anchor, got.File.Property, got.File.Name, data),
	}}
}
